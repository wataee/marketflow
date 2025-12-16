package exchange

import (
	"bufio"
	"context"
	"fmt"
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
	return &TCPExchange{name: name, host: host, port: port, log: log}
}

func (t *TCPExchange) Name() string { return t.name }

func (t *TCPExchange) Connect(ctx context.Context) error {
	addr := fmt.Sprintf("%s:%d", t.host, t.port)
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return err
	}
	t.conn = conn
	t.log.Info("connected to exchange", "exchange", t.name)
	return nil
}

func (t *TCPExchange) Subscribe(symbols []string) error {
	// tcp source sends all data automatically
	return nil
}

// ReadPrices читает данные строки: SYMBOL,PRICE\n
func (t *TCPExchange) ReadPrices(ctx context.Context) (<-chan model.PriceUpdate, <-chan error) {
	out := make(chan model.PriceUpdate)
	errCh := make(chan error)
	ctx, cancel := context.WithCancel(ctx)
	t.cancel = cancel

	go func() {
		defer close(out)
		defer close(errCh)
		reader := bufio.NewReader(t.conn)
		for {
			select {
			case <-ctx.Done():
				return
			default:
				line, err := reader.ReadString('\n')
				if err != nil {
					errCh <- err
					return
				}
				line = strings.TrimSpace(line)
				parts := strings.Split(line, ",")
				if len(parts) != 2 {
					continue
				}
				priceVal, err := strconv.ParseFloat(parts[1], 64)
				if err != nil {
					continue
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
	if t.cancel != nil {
		t.cancel()
	}
	if t.conn != nil {
		return t.conn.Close()
	}
	return nil
}
