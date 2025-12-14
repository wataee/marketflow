package exchange

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"marketflow/internal/domain/model"
	"net"
	"time"
)

type TCPExchange struct {
	name    string
	address string
	conn    net.Conn
	logger  *slog.Logger
}

func NewTCPExchange(name, host string, port int, logger *slog.Logger) *TCPExchange {
	return &TCPExchange{
		name:    name,
		address: fmt.Sprintf("%s:%d", host, port),
		logger:  logger,
	}
}

func (e *TCPExchange) Connect(ctx context.Context) error {
	conn, err := net.Dial("tcp", e.address)
	if err != nil {
		return fmt.Errorf("failed to connect to %s: %w", e.name, err)
	}
	e.conn = conn
	e.logger.Info("connected to exchange", "exchange", e.name, "address", e.address)
	return nil
}

func (e *TCPExchange) Subscribe(symbols []string) error {
	// TODO: Send subscription message if needed
	return nil
}

func (e *TCPExchange) ReadPrices(ctx context.Context) (<-chan model.PriceUpdate, <-chan error) {
	priceCh := make(chan model.PriceUpdate)
	errCh := make(chan error, 1)

	go func() {
		defer close(priceCh)
		defer close(errCh)

		scanner := bufio.NewScanner(e.conn)
		for scanner.Scan() {
			select {
			case <-ctx.Done():
				return
			default:
				line := scanner.Text()
				var update model.PriceUpdate
				if err := json.Unmarshal([]byte(line), &update); err != nil {
					e.logger.Error("failed to parse price", "error", err, "line", line)
					continue
				}
				update.Exchange = e.name
				update.Timestamp = time.Now()
				priceCh <- update
			}
		}

		if err := scanner.Err(); err != nil {
			errCh <- err
		}
	}()

	return priceCh, errCh
}

func (e *TCPExchange) Close() error {
	if e.conn != nil {
		return e.conn.Close()
	}
	return nil
}

func (e *TCPExchange) Name() string {
	return e.name
}
