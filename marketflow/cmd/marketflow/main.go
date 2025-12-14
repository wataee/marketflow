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
	port1     = flag.Int("port", 0, "Port number")
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

	cfg, err := config.Load("configs/config.yaml")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	if *port1 != 0 {
		cfg.Server.Port = *port1
	}

	log := logger.New(cfg.Logging.Level, cfg.Logging.Format)
	log.Info("starting marketflow", "version", "1.0.0")

	postgresAdapter, err := storage.NewPostgresAdapter(cfg.PostgresDSN())
	if err != nil {
		log.Error("failed to initialize postgres", "error", err)
		os.Exit(1)
	}
	defer postgresAdapter.Close()

	if err := postgresAdapter.InitSchema(context.Background()); err != nil {
		log.Error("failed to initialize schema", "error", err)
		os.Exit(1)
	}

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

	priceUseCase := usecase.NewPriceUseCase(postgresAdapter, redisAdapter)
	modeService := service.NewModeService(log)
	aggregationService := service.NewAggregationService(redisAdapter, postgresAdapter, log, cfg.TradingPairs)

	aggregationService.Start(context.Background(), cfg.DataRetention.AggregationInterval)
	defer aggregationService.Stop()

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
	mux.HandleFunc("POST /mode/test", modeHandler.SwitchToTest)
	mux.HandleFunc("POST /mode/live", modeHandler.SwitchToLive)
	mux.HandleFunc("GET /health", healthHandler.Check)

	srv := server.NewServer(cfg.Server.Port, mux, log)
	app.server = srv

	go func() {
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
	<-sigCh

	log.Info("shutting down gracefully")
	app.shutdown()
}

func (a *App) startDataProcessing(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	var exchanges []port.ExchangePort

	if a.modeService.GetCurrentMode() == model.LiveMode {
		for _, exCfg := range a.config.Exchanges {
			if !exCfg.Enabled {
				continue
			}
			ex := exchange.NewTCPExchange(exCfg.Name, exCfg.Host, exCfg.Port, a.logger)
			exchanges = append(exchanges, ex)
		}
	} else {
		gen := generator.NewTestGenerator("test-generator", a.config.TradingPairs, a.logger)
		exchanges = append(exchanges, gen)
	}

	var priceChannels []<-chan model.PriceUpdate

	for _, ex := range exchanges {
		if err := ex.Connect(ctx); err != nil {
			a.logger.Error("failed to connect", "exchange", ex.Name(), "error", err)
			continue
		}

		priceCh, errCh := ex.ReadPrices(ctx)
		priceChannels = append(priceChannels, priceCh)

		go func(name string, errCh <-chan error) {
			for err := range errCh {
				a.logger.Error("exchange error", "exchange", name, "error", err)
				a.reconnectExchange(ctx, ex)
			}
		}(ex.Name(), errCh)
	}

	if len(priceChannels) == 0 {
		return fmt.Errorf("no exchanges connected")
	}

	a.exchanges = exchanges

	mergedCh := fanin.FanIn(priceChannels...)

	workerPool := worker.NewPool(
		a.config.Workers.PerExchange*len(exchanges),
		a.cacheAdapter,
		a.storageAdapter,
		a.logger,
	)
	processedCh := workerPool.Start(ctx, mergedCh)

	go func() {
		for range processedCh {
		}
	}()

	a.logger.Info("data processing started",
		"exchanges", len(exchanges),
		"workers", a.config.Workers.PerExchange*len(exchanges))

	return nil
}

func (a *App) reconnectExchange(ctx context.Context, ex port.ExchangePort) {
	backoff := time.Second
	maxBackoff := 30 * time.Second

	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
			a.logger.Info("attempting to reconnect", "exchange", ex.Name())
			if err := ex.Connect(ctx); err == nil {
				a.logger.Info("reconnected successfully", "exchange", ex.Name())

				priceCh, errCh := ex.ReadPrices(ctx)
				go func() {
					for range priceCh {
					}
				}()

				go func() {
					for err := range errCh {
						a.logger.Error("exchange error after reconnect", "exchange", ex.Name(), "error", err)
						a.reconnectExchange(ctx, ex)
					}
				}()
				return
			}

			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
		}
	}
}

func (a *App) switchMode(ctx context.Context, newMode model.DataMode) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.logger.Info("switching mode", "from", a.modeService.GetCurrentMode(), "to", newMode)

	if a.cancel != nil {
		a.cancel()
	}

	for _, ex := range a.exchanges {
		if err := ex.Close(); err != nil {
			a.logger.Error("failed to close exchange", "exchange", ex.Name(), "error", err)
		}
	}
	a.exchanges = nil

	if err := a.modeService.SwitchMode(ctx, newMode); err != nil {
		return err
	}

	newCtx, cancel := context.WithCancel(context.Background())
	a.cancel = cancel

	time.Sleep(500 * time.Millisecond)

	return a.startDataProcessing(newCtx)
}

func (a *App) shutdown() {
	if a.cancel != nil {
		a.cancel()
	}

	for _, ex := range a.exchanges {
		if err := ex.Close(); err != nil {
			a.logger.Error("failed to close exchange", "exchange", ex.Name(), "error", err)
		}
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := a.server.Shutdown(shutdownCtx); err != nil {
		a.logger.Error("shutdown error", "error", err)
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