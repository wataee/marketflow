package exchange

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"marketflow/internal/domain/model"
	"marketflow/internal/domain/port"
	"net"
	"strconv"
	"strings"
	"sync" // <-- ВАЖНО: Добавляем мьютекс для потокобезопасности
	"time"
)

// TCPExchange реализует интерфейс port.ExchangePort.
type TCPExchange struct {
	name   string
	host   string
	port   int
	conn   net.Conn
	log    *slog.Logger
	cancel context.CancelFunc // Для отмены горутины ReadPrices
	mu     sync.RWMutex       // <-- КРИТИЧЕСКИЙ ЭЛЕМЕНТ: Защищает conn и cancel
}

func NewTCPExchange(name, host string, port int, log *slog.Logger) port.ExchangePort {
	return &TCPExchange{
		name: name,
		host: host,
		port: port,
		log:  log,
	}
}

func (t *TCPExchange) Name() string {
	return t.name
}

// Connect устанавливает TCP-соединение с использованием контекста.
func (t *TCPExchange) Connect(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	addr := net.JoinHostPort(t.host, strconv.Itoa(t.port))
	t.log.Info("connecting to TCP exchange", "exchange", t.name, "addr", addr)

	// Используем Dialer с Context для контроля таймаутов и прерывания
	dialer := net.Dialer{
		Timeout: 5 * time.Second, 
	}

	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		t.log.Error("failed to connect to TCP exchange", "exchange", t.name, "addr", addr, "error", err)
		return err
	}

	// Если соединение уже было, закрываем его перед заменой
	if t.conn != nil {
		t.conn.Close()
	}

	t.conn = conn
	t.log.Info("connected to TCP exchange successfully", "exchange", t.name, "addr", addr)
	return nil
}

func (t *TCPExchange) Subscribe(symbols []string) error {
	// Для тестового сервера подписка не требуется
	return nil
}

// rawPriceUpdate - структура для парсинга входящего JSON-сообщения.
type rawPriceUpdate struct {
	Symbol    string  `json:"symbol"`
	Price     float64 `json:"price"`
	Timestamp int64   `json:"timestamp"` // Таймштамп в миллисекундах
}

// ReadPrices запускает чтение данных из соединения и отправляет их в выходной канал.
func (t *TCPExchange) ReadPrices(ctx context.Context) (<-chan model.PriceUpdate, <-chan error) {
	out := make(chan model.PriceUpdate)
	errCh := make(chan error)

	// Создаем дочерний контекст для контроля цикла чтения
	readCtx, cancel := context.WithCancel(ctx)
	
	t.mu.Lock()
	t.cancel = cancel
	currentConn := t.conn // Безопасно копируем ссылку на соединение
	t.mu.Unlock()

	if currentConn == nil {
		t.log.Error("cannot start reading, connection is nil", "exchange", t.name)
		close(out)
		close(errCh)
		cancel()
		return out, errCh 
	}

	t.log.Info("starting to read prices", "exchange", t.name)

	go func() {
		defer close(out)
		defer close(errCh)
		
		// Этот defer гарантирует, что при выходе горутины ресурсы будут освобождены.
		defer func() {
			t.mu.Lock()
			// Отменяем контекст, если горутина завершилась сама
			if t.cancel != nil {
				t.cancel()
				t.cancel = nil
			}
			t.mu.Unlock()
			// Закрытие соединения: это вызовет ошибку в ReadString, если оно еще активно.
			currentConn.Close()
		}()

		reader := bufio.NewReader(currentConn)
		lineCount := 0
		errorCount := 0

		for {
			select {
			case <-readCtx.Done(): // Используем readCtx
				t.log.Info("read prices stopped by context cancellation", "exchange", t.name, "lines_read", lineCount, "errors", errorCount)
				return
			default:
				
				// Чтение до символа новой строки.
				line, err := reader.ReadString('\n')
				if err != nil {
					t.log.Error("error reading from TCP exchange, triggering reconnect", "exchange", t.name, "error", err)
					// Отправляем ошибку в errCh, чтобы main.go запустил reconnectExchange
					select {
					case errCh <- fmt.Errorf("read error: %w", err):
					default: // Не блокируем
					}
					return // Выход из горутины чтения
				}

				line = strings.TrimSpace(line)
				if line == "" {
					continue
				}

				var raw rawPriceUpdate
				if err := json.Unmarshal([]byte(line), &raw); err != nil {
					t.log.Warn("invalid line format (JSON error)", "exchange", t.name, "line_len", len(line), "error", err)
					errorCount++
					continue
				}

				// Обработка Timestamp: используем текущее время, если Timestamp не пришел (равен 0)
				ts := time.UnixMilli(raw.Timestamp)
				// Проверяем, что таймштамп не нулевой и не слишком старый
				if raw.Timestamp == 0 || ts.Before(time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)) { 
					ts = time.Now()
					t.log.Debug("using current time for price (missing or old timestamp)", "exchange", t.name, "symbol", raw.Symbol)
				}
				
				priceUpdate := model.PriceUpdate{
					Symbol:   raw.Symbol,
					Exchange: t.name,
					Price:    raw.Price,
					Timestamp: ts,
				}

				lineCount++

				// Отправка в выходной канал, с обязательной проверкой контекста
				select {
				case out <- priceUpdate:
				case <-readCtx.Done():
					return 
				}
			}
		}
	}()

	return out, errCh
}

// Close закрывает соединение и отменяет горутину чтения.
func (t *TCPExchange) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	
	t.log.Info("closing TCP exchange", "exchange", t.name)

	// 1. Отмена контекста для мягкой остановки горутины ReadPrices
	if t.cancel != nil {
		t.cancel()
		t.cancel = nil // Устанавливаем в nil, чтобы предотвратить повторную отмену
	}
	
	// 2. Закрытие соединения
	if t.conn != nil {
		err := t.conn.Close()
		t.conn = nil // Устанавливаем в nil, чтобы сигнализировать о закрытии
		if err != nil {
			t.log.Error("error closing connection", "exchange", t.name, "error", err)
			return err
		}
		t.log.Info("TCP exchange closed successfully", "exchange", t.name)
	}

	return nil
}