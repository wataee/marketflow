package exchange

import (
	"bufio"
	"context"
	"log/slog"
	"marketflow/internal/domain/model"
	"marketflow/internal/domain/port"
	"net"
	"strconv"
	"strings"
	"time"
)

type TCPExchange struct {
	name   string
	host   string
	port   int
	conn   net.Conn
	log    *slog.Logger
	cancel context.CancelFunc
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
	addr := net.JoinHostPort(t.host, strconv.Itoa(t.port))
	t.log.Info("connecting to TCP exchange", "exchange", t.name, "addr", addr)

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.log.Error("failed to connect to TCP exchange", "exchange", t.name, "addr", addr, "error", err)
		return err
	}

	t.conn = conn
	t.log.Info("connected to TCP exchange successfully", "exchange", t.name, "addr", addr)
	return nil
}

func (t *TCPExchange) Subscribe(symbols []string) error {
	return nil
}

func (t *TCPExchange) ReadPrices(ctx context.Context) (<-chan model.PriceUpdate, <-chan error) {
	out := make(chan model.PriceUpdate)
	errCh := make(chan error)
	ctx, cancel := context.WithCancel(ctx)
	t.cancel = cancel

	t.log.Info("starting to read prices", "exchange", t.name)

	go func() {
		defer close(out)
		defer close(errCh)

		reader := bufio.NewReader(t.conn)
		lineCount := 0
		errorCount := 0

		for {
			select {
			case <-ctx.Done():
				t.log.Info("read prices stopped", "exchange", t.name, "lines_read", lineCount, "errors", errorCount)
				return
			default:
				line, err := reader.ReadString('\n')
				if err != nil {
					t.log.Error("error reading from TCP exchange", "exchange", t.name, "error", err)
					errCh <- err
					return
				}

				line = strings.TrimSpace(line)
				if line == "" {
					continue
				}

				parts := strings.Split(line, ",")
				if len(parts) != 2 {
					t.log.Warn("invalid line format", "exchange", t.name, "line", line)
					errorCount++
					continue
				}

				priceVal, err := strconv.ParseFloat(parts[1], 64)
				if err != nil {
					t.log.Warn("failed to parse price", "exchange", t.name, "price_str", parts[1], "error", err)
					errorCount++
					continue
				}

				lineCount++
				if lineCount%100 == 0 {
					t.log.Debug("prices read progress", "exchange", t.name, "count", lineCount)
				}

				out <- model.PriceUpdate{
					Symbol:    parts[0],
					Exchange:  t.name,
					Price:     priceVal,
					Timestamp: time.Now(),
				}
			}
		}
	}()

	return out, errCh
}

func (t *TCPExchange) Close() error {
	t.log.Info("closing TCP exchange", "exchange", t.name)

	if t.cancel != nil {
		t.cancel()
	}

	if t.conn != nil {
		err := t.conn.Close()
		if err != nil {
			t.log.Error("error closing connection", "exchange", t.name, "error", err)
			return err
		}
		t.log.Info("TCP exchange closed", "exchange", t.name)
	}

	return nil
}
