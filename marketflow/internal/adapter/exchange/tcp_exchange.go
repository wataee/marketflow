package exchange

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"marketflow/internal/domain/model"
	"marketflow/internal/domain/port"
)

type TCPExchange struct {
	name   string
	host   string
	port   int
	conn   net.Conn
	log    *slog.Logger
	cancel context.CancelFunc
	mu     sync.RWMutex
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

func (t *TCPExchange) Connect(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	addr := net.JoinHostPort(t.host, strconv.Itoa(t.port))
	t.log.Info("connecting to TCP exchange", "exchange", t.name, "addr", addr)

	dialer := net.Dialer{
		Timeout: 5 * time.Second,
	}

	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		t.log.Error("failed to connect to TCP exchange", "exchange", t.name, "addr", addr, "error", err)
		return err
	}

	if t.conn != nil {
		t.conn.Close()
	}

	t.conn = conn
	t.log.Info("connected to TCP exchange successfully", "exchange", t.name, "addr", addr)
	return nil
}

func (t *TCPExchange) Subscribe(symbols []string) error {
	return nil
}

type rawPriceUpdate struct {
	Symbol    string  `json:"symbol"`
	Price     float64 `json:"price"`
	Timestamp int64   `json:"timestamp"`
}

func (t *TCPExchange) ReadPrices(ctx context.Context) (<-chan model.PriceUpdate, <-chan error) {
	out := make(chan model.PriceUpdate)
	errCh := make(chan error)

	readCtx, cancel := context.WithCancel(ctx)

	t.mu.Lock()
	t.cancel = cancel
	currentConn := t.conn
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

		defer func() {
			t.mu.Lock()
			if t.cancel != nil {
				t.cancel()
				t.cancel = nil
			}
			t.mu.Unlock()
			currentConn.Close()
		}()

		reader := bufio.NewReader(currentConn)
		lineCount := 0
		errorCount := 0

		for {
			select {
			case <-readCtx.Done():
				t.log.Info("read prices stopped by context cancellation", "exchange", t.name, "lines_read", lineCount, "errors", errorCount)
				return
			default:
				line, err := reader.ReadString('\n')
				if err != nil {
					t.log.Error("error reading from TCP exchange, triggering reconnect", "exchange", t.name, "error", err)
					select {
					case errCh <- fmt.Errorf("read error: %w", err):
					default:
					}
					return
				}

				line = strings.TrimSpace(line)
				if line == "" {
					continue
				}

				var priceUpdate model.PriceUpdate
				if parsed, ok := t.parseJSON(line); ok {
					priceUpdate = parsed
				} else if parsed, ok := t.parseCSV(line); ok {
					priceUpdate = parsed
				} else {
					t.log.Warn("invalid line format (neither JSON nor CSV)", "exchange", t.name, "line_preview", truncate(line, 50))
					errorCount++
					continue
				}

				lineCount++
				if lineCount%100 == 0 {
					t.log.Debug("prices read progress", "exchange", t.name, "count", lineCount)
				}

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

func (t *TCPExchange) parseJSON(line string) (model.PriceUpdate, bool) {
	var raw rawPriceUpdate
	if err := json.Unmarshal([]byte(line), &raw); err != nil {
		return model.PriceUpdate{}, false
	}

	ts := time.UnixMilli(raw.Timestamp)
	if raw.Timestamp == 0 || ts.Before(time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)) {
		ts = time.Now()
	}

	return model.PriceUpdate{
		Symbol:    raw.Symbol,
		Exchange:  t.name,
		Price:     raw.Price,
		Timestamp: ts,
	}, true
}

func (t *TCPExchange) parseCSV(line string) (model.PriceUpdate, bool) {
	parts := strings.Split(line, ",")
	if len(parts) != 2 {
		return model.PriceUpdate{}, false
	}

	symbol := strings.TrimSpace(parts[0])
	priceStr := strings.TrimSpace(parts[1])

	price, err := strconv.ParseFloat(priceStr, 64)
	if err != nil {
		return model.PriceUpdate{}, false
	}

	return model.PriceUpdate{
		Symbol:    symbol,
		Exchange:  t.name,
		Price:     price,
		Timestamp: time.Now(),
	}, true
}

func (t *TCPExchange) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.log.Info("closing TCP exchange", "exchange", t.name)

	if t.cancel != nil {
		t.cancel()
		t.cancel = nil
	}

	if t.conn != nil {
		err := t.conn.Close()
		t.conn = nil
		if err != nil {
			t.log.Error("error closing connection", "exchange", t.name, "error", err)
			return err
		}
		t.log.Info("TCP exchange closed successfully", "exchange", t.name)
	}

	return nil
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
