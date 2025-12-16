package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"marketflow/internal/adapter/cache"
	"marketflow/internal/adapter/exchange"
	"marketflow/internal/adapter/generator"
	"marketflow/internal/adapter/handler"
	"marketflow/internal/adapter/storage"
	"marketflow/internal/application/service"
	"marketflow/internal/application/usecase"
	"marketflow/internal/concurrency/fanin"
	"marketflow/internal/concurrency/worker"
	"marketflow/internal/domain/model"
	"marketflow/internal/domain/port"
	"marketflow/internal/infrastructure/config"
	"marketflow/internal/infrastructure/logger"
	"marketflow/internal/infrastructure/server"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

var (
	portFlag = flag.Int("port", 0, "Port number")
	helpFlag = flag.Bool("help", false, "Show help")
)

type App struct {
	config             *config.Config
	logger             *slog.Logger
	server             *server.Server
	storageAdapter     port.StoragePort
	cacheAdapter       port.CachePort
	aggregationService *service.AggregationService
	modeService        *service.ModeService
	exchanges          []port.ExchangePort
	cancel             context.CancelFunc
	mu                 sync.RWMutex
}

func main() {
	flag.Parse()

	if *helpFlag {
		printUsage()
		os.Exit(0)
	}

	cfg, err := config.LoadConfig("configs/config.json")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	if *portFlag != 0 {
		cfg.Server.Port = *portFlag
	}

	log := logger.New(cfg.Logging.Level, cfg.Logging.Format)
	log.Info("starting marketflow", "version", "1.0.0", "port", cfg.Server.Port)

	postgresAdapter, err := storage.NewPostgresAdapter(cfg.PostgresDSN())
	if err != nil {
		log.Error("failed to initialize postgres", "error", err)
		os.Exit(1)
	}
	defer postgresAdapter.Close()
	log.Info("postgres adapter initialized successfully")

	if err := postgresAdapter.InitSchema(context.Background()); err != nil {
		log.Error("failed to initialize schema", "error", err)
		os.Exit(1)
	}
	log.Info("database schema initialized successfully")

	redisAdapter, err := cache.NewRedisAdapter(
		cfg.RedisAddr(),
		cfg.Redis.Password,
		cfg.Redis.DB,
		cfg.DataRetention.RedisTTL,
	)
	if err != nil {
		log.Error("failed to initialize redis", "error", err)
		os.Exit(1)
	}
	defer redisAdapter.Close()
	log.Info("redis adapter initialized successfully")

	priceUseCase := usecase.NewPriceUseCase(postgresAdapter, redisAdapter)
	modeService := service.NewModeService(log)
	aggregationService := service.NewAggregationService(redisAdapter, postgresAdapter, log)

	aggregationService.SetTradingPairs(cfg.TradingPairs)
	log.Info("trading pairs configured", "pairs", cfg.TradingPairs)

	aggregationService.Start(context.Background(), cfg.DataRetention.AggregationInterval)
	defer aggregationService.Stop()
	log.Info("aggregation service started", "interval", cfg.DataRetention.AggregationInterval)

	app := &App{
		config:             cfg,
		logger:             log,
		storageAdapter:     postgresAdapter,
		cacheAdapter:       redisAdapter,
		aggregationService: aggregationService,
		modeService:        modeService,
	}

	priceHandler := handler.NewPriceHandler(priceUseCase, log)
	modeHandler := handler.NewModeHandler(modeService, app.switchMode, log)
	healthHandler := handler.NewHealthHandler(postgresAdapter, redisAdapter, log)

	mux := http.NewServeMux()

	mux.HandleFunc("/prices/latest/", priceHandler.GetLatestPrice)
	mux.HandleFunc("/prices/highest/", priceHandler.GetHighestPrice)
	mux.HandleFunc("/prices/lowest/", priceHandler.GetLowestPrice)
	mux.HandleFunc("/prices/average/", priceHandler.GetAveragePrice)
	mux.HandleFunc("/mode/test", modeHandler.SwitchToTest)
	mux.HandleFunc("/mode/live", modeHandler.SwitchToLive)
	mux.HandleFunc("/health", healthHandler.Check)

	log.Info("routes registered successfully")

	srv := server.NewServer(cfg.Server.Port, mux, log)
	app.server = srv

	go func() {
		log.Info("starting http server", "port", cfg.Server.Port)
		if err := srv.Start(); err != nil && err != http.ErrServerClosed {
			log.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())
	app.cancel = cancel

	if err := app.startDataProcessing(ctx); err != nil {
		log.Error("failed to start data processing", "error", err)
		os.Exit(1)
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	sig := <-sigCh
	log.Info("received shutdown signal", "signal", sig)

	log.Info("shutting down gracefully")
	app.shutdown()
}

func (a *App) startDataProcessing(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	var exchanges []port.ExchangePort

	currentMode := a.modeService.GetCurrentMode()
	a.logger.Info("starting data processing", "mode", currentMode)

	if currentMode == model.LiveMode {
		for _, exCfg := range a.config.Exchanges {
			if !exCfg.Enabled {
				a.logger.Info("exchange disabled, skipping", "exchange", exCfg.Name)
				continue
			}
			ex := exchange.NewTCPExchange(exCfg.Name, exCfg.Host, exCfg.Port, a.logger)
			exchanges = append(exchanges, ex)
		}
		a.logger.Info("live mode exchanges prepared", "count", len(exchanges))
	} else {
		gen := generator.NewTestGenerator("test-generator", a.config.TradingPairs, a.logger)
		exchanges = append(exchanges, gen)
		a.logger.Info("test mode generator prepared")
	}

	var priceChannels []<-chan model.PriceUpdate

	for _, ex := range exchanges {
		a.logger.Info("connecting to exchange", "exchange", ex.Name())
		if err := ex.Connect(ctx); err != nil {
			a.logger.Error("failed to connect", "exchange", ex.Name(), "error", err)
			continue
		}
		a.logger.Info("connected successfully", "exchange", ex.Name())

		priceCh, errCh := ex.ReadPrices(ctx)
		priceChannels = append(priceChannels, priceCh)

		go func(name string, errCh <-chan error, ex port.ExchangePort) {
			for err := range errCh {
				a.logger.Error("exchange error", "exchange", name, "error", err)
				go a.reconnectExchange(ctx, ex)
			}
		}(ex.Name(), errCh, ex)
	}

	if len(priceChannels) == 0 {
		return fmt.Errorf("no exchanges connected")
	}

	a.exchanges = exchanges

	mergedCh := fanin.FanIn(priceChannels...)
	a.logger.Info("fan-in channel created", "sources", len(priceChannels))

	workerCount := a.config.Workers.PerExchange * len(exchanges)
	workerPool := worker.NewPool(
		workerCount,
		a.cacheAdapter,
		a.storageAdapter,
		a.logger,
	)
	processedCh := workerPool.Start(ctx, mergedCh)
	a.logger.Info("worker pool started", "workers", workerCount)

	go func() {
		count := 0
		for range processedCh {
			count++
			if count%100 == 0 {
				a.logger.Debug("processed prices", "count", count)
			}
		}
		a.logger.Info("processed channel closed", "total_processed", count)
	}()

	a.logger.Info("data processing started successfully",
		"exchanges", len(exchanges),
		"workers", workerCount)

	return nil
}

func (a *App) reconnectExchange(ctx context.Context, ex port.ExchangePort) {
	backoff := time.Second
	maxBackoff := 30 * time.Second
	attempt := 1

	for {
		select {
		case <-ctx.Done():
			a.logger.Info("reconnect cancelled", "exchange", ex.Name())
			return
		case <-time.After(backoff):
			a.logger.Info("attempting to reconnect", "exchange", ex.Name(), "attempt", attempt)
			if err := ex.Connect(ctx); err == nil {
				a.logger.Info("reconnected successfully", "exchange", ex.Name(), "attempt", attempt)

				priceCh, errCh := ex.ReadPrices(ctx)
				
				go func() {
					count := 0
					for range priceCh {
						count++
					}
					a.logger.Info("reconnected price channel closed", "exchange", ex.Name(), "count", count)
				}()

				go func() {
					for err := range errCh {
						a.logger.Error("exchange error after reconnect", "exchange", ex.Name(), "error", err)
						go a.reconnectExchange(ctx, ex)
					}
				}()
				return
			} else {
				a.logger.Warn("reconnect failed", "exchange", ex.Name(), "attempt", attempt, "error", err, "next_backoff", backoff*2)
			}

			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
			attempt++
		}
	}
}

func (a *App) switchMode(ctx context.Context, newMode model.DataMode) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	currentMode := a.modeService.GetCurrentMode()
	a.logger.Info("switching mode", "from", currentMode, "to", newMode)

	if currentMode == newMode {
		a.logger.Info("already in requested mode", "mode", newMode)
		return nil
	}

	if a.cancel != nil {
		a.logger.Info("cancelling current context")
		a.cancel()
	}

	for _, ex := range a.exchanges {
		a.logger.Info("closing exchange", "exchange", ex.Name())
		if err := ex.Close(); err != nil {
			a.logger.Error("failed to close exchange", "exchange", ex.Name(), "error", err)
		}
	}
	a.exchanges = nil
	a.logger.Info("all exchanges closed")

	if err := a.modeService.SwitchMode(ctx, newMode); err != nil {
		a.logger.Error("failed to switch mode in service", "error", err)
		return err
	}

	newCtx, cancel := context.WithCancel(context.Background())
	a.cancel = cancel

	time.Sleep(500 * time.Millisecond)
	a.logger.Info("restarting data processing with new mode", "mode", newMode)

	return a.startDataProcessing(newCtx)
}

func (a *App) shutdown() {
	a.logger.Info("starting shutdown sequence")

	if a.cancel != nil {
		a.cancel()
	}

	for _, ex := range a.exchanges {
		a.logger.Info("closing exchange during shutdown", "exchange", ex.Name())
		if err := ex.Close(); err != nil {
			a.logger.Error("failed to close exchange", "exchange", ex.Name(), "error", err)
		}
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := a.server.Shutdown(shutdownCtx); err != nil {
		a.logger.Error("shutdown error", "error", err)
	} else {
		a.logger.Info("server shut down successfully")
	}

	a.logger.Info("shutdown complete")
}

func printUsage() {
	fmt.Println("Usage:")
	fmt.Println("  marketflow [--port <N>]")
	fmt.Println("  marketflow --help")
	fmt.Println()
	fmt.Println("Options:")
	fmt.Println("  --port N     Port number")
}