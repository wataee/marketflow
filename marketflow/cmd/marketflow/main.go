package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"marketflow/internal/adapter/cache"
	"marketflow/internal/adapter/exchange"
	"marketflow/internal/adapter/generator"
	"marketflow/internal/adapter/handler"
	"marketflow/internal/adapter/storage" // <-- Пакет storage определен
	"marketflow/internal/application/service"
	"marketflow/internal/application/usecase"
	"marketflow/internal/concurrency/fanin"
	"marketflow/internal/concurrency/worker"
	"marketflow/internal/domain/model"
	"marketflow/internal/domain/port"
	"marketflow/internal/infrastructure/config"
	"marketflow/internal/infrastructure/logger"
	"marketflow/internal/infrastructure/server"
)

var (
	portFlag = flag.Int("port", 0, "Port number")
	helpFlag = flag.Bool("help", false, "Show help")
)

// App представляет главный контейнер приложения и его зависимости
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

	// --- Инициализация Postgres ---
	postgresAdapter, err := storage.NewPostgresAdapter(cfg.PostgresDSN()) // <-- Использование пакета storage
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

	// --- Инициализация Redis ---
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

	// --- Инициализация сервисов и UseCase ---
	priceUseCase := usecase.NewPriceUseCase(postgresAdapter, redisAdapter)
	modeService := service.NewModeService(log)
	aggregationService := service.NewAggregationService(redisAdapter, postgresAdapter, log)

	aggregationService.SetTradingPairs(cfg.TradingPairs)
	log.Info("trading pairs configured", "pairs", cfg.TradingPairs)

	// Запуск Aggregation Service (для Min/Max/Avg в PostgreSQL)
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

	// --- Инициализация HTTP хендлеров ---
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

	// --- Запуск HTTP сервера ---
	go func() {
		log.Info("starting http server", "port", cfg.Server.Port)
		if err := srv.Start(); err != nil && err != http.ErrServerClosed {
			log.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	// --- Запуск обработки данных ---
	ctx, cancel := context.WithCancel(context.Background())
	app.cancel = cancel

	if err := app.startDataProcessing(ctx); err != nil {
		log.Error("failed to start data processing", "error", err)
		app.shutdown()
		os.Exit(1)
	}

	// --- Graceful Shutdown ---
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
	return a.startDataProcessingInternal(ctx)
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

				// !!! ВАЖНАЯ АРХИТЕКТУРНАЯ ЗАМЕТКА !!!
				// При успешном переподключении мы получаем НОВЫЕ каналы (priceCh, errCh).
				// Новый priceCh НЕ подключен к Fan-In. В данной архитектуре
				// корректное добавление нового потока данных без перезапуска Fan-In сложно.
				// Сейчас мы просто перезапускаем горутину обработки ошибок.

				_, errCh := ex.ReadPrices(ctx) // Получаем новые каналы (priceCh игнорируем, т.к. не можем добавить в Fan-In)

				go func(ex port.ExchangePort) {
					for err := range errCh {
						a.logger.Error("exchange error after reconnect", "exchange", ex.Name(), "error", err)
						go a.reconnectExchange(ctx, ex)
						return // Важно выйти после повторного вызова reconnect
					}
				}(ex)

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
	if currentMode == newMode {
		return nil
	}

	// 1. Останавливаем старый контекст
	if a.cancel != nil {
		a.cancel()
	}

	// 2. Закрываем старые соединения
	for _, ex := range a.exchanges {
		_ = ex.Close()
	}
	a.exchanges = nil

	// 3. Обновляем режим в сервисе
	if err := a.modeService.SwitchMode(ctx, newMode); err != nil {
		return err
	}

	// 4. Запускаем новую обработку с НОВЫМ контекстом
	newCtx, cancel := context.WithCancel(context.Background())
	a.cancel = cancel

	// Небольшая пауза, чтобы старые горутины успели закрыть сокеты
	time.Sleep(100 * time.Millisecond)

	return a.startDataProcessingInternal(newCtx)
}

func (a *App) shutdown() {
	a.logger.Info("starting shutdown sequence")

	// 1. Отмена контекста для остановки всей обработки данных
	if a.cancel != nil {
		a.cancel()
	}

	// 2. Закрытие всех Exchange (если еще не закрыты)
	for _, ex := range a.exchanges {
		a.logger.Info("closing exchange during shutdown", "exchange", ex.Name())
		if err := ex.Close(); err != nil {
			a.logger.Error("failed to close exchange", "exchange", ex.Name(), "error", err)
		}
	}

	// 3. Graceful Shutdown HTTP сервера
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := a.server.Shutdown(shutdownCtx); err != nil {
		a.logger.Error("server shutdown error", "error", err)
	} else {
		a.logger.Info("server shut down successfully")
	}

	a.logger.Info("shutdown complete")
}

func (a *App) startDataProcessingInternal(ctx context.Context) error {
	var exchanges []port.ExchangePort

	currentMode := a.modeService.GetCurrentMode()
	a.logger.Info("starting data processing", "mode", currentMode)

	if currentMode == model.LiveMode {
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

		go func(name string, eCh <-chan error, exPort port.ExchangePort) {
			for err := range eCh {
				a.logger.Error("exchange error", "exchange", name, "error", err)
				go a.reconnectExchange(ctx, exPort)
			}
		}(ex.Name(), errCh, ex)
	}

	if len(priceChannels) == 0 {
		return fmt.Errorf("no exchanges connected")
	}

	a.exchanges = exchanges
	mergedCh := fanin.FanIn(priceChannels...)
	workerCount := a.config.Workers.PerExchange * len(exchanges)
	workerPool := worker.NewPool(workerCount, a.cacheAdapter, a.storageAdapter, a.logger)

	processedCh := workerPool.Start(ctx, mergedCh)

	go func() {
		for range processedCh {
			// Очищаем канал, чтобы не было deadlock
		}
	}()

	return nil
}

func printUsage() {
	fmt.Println("Usage:")
	fmt.Println("  marketflow [--port <N>]")
	fmt.Println("  marketflow --help")
	fmt.Println()
	fmt.Println("Options:")
	fmt.Println("  --port N      Port number")
}
