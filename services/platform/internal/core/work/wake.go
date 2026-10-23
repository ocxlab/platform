package work

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"ocp.local/platform/internal/core/capability"
)

type WakeSignal string

const (
	Listening WakeSignal = "listening"
	Changed   WakeSignal = "changed"
)

// WakeSource sends coalescible hints; it never owns or acknowledges durable work.
// Listening is sent only after LISTEN succeeds, including after reconnection.
type WakeSource interface {
	Run(context.Context, chan<- WakeSignal) error
}

type PostgresWake struct {
	DSN     string
	Domains []capability.Domain
}

func signalWake(out chan<- WakeSignal, signal WakeSignal) {
	select {
	case out <- signal:
	default:
	}
}

func (p PostgresWake) Run(ctx context.Context, out chan<- WakeSignal) error {
	backoff := time.Second
	for ctx.Err() == nil {
		connected := p.listen(ctx, out)
		if connected {
			backoff = time.Second
		}
		timer := time.NewTimer(backoff)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil
		case <-timer.C:
		}
		backoff = min(30*time.Second, backoff*2)
	}
	return nil
}

func (p PostgresWake) listen(ctx context.Context, out chan<- WakeSignal) bool {
	connect, cancel := context.WithTimeout(ctx, 5*time.Second)
	config, err := pgx.ParseConfig(p.DSN)
	if err != nil {
		cancel()
		return false
	}
	config.RuntimeParams["application_name"] = "ocp-work-wake"
	conn, err := pgx.ConnectConfig(connect, config)
	if err == nil {
		_, err = conn.Exec(connect, "LISTEN ocp_work_changed")
	}
	cancel()
	if conn != nil {
		defer func() {
			closeCtx, closeCancel := context.WithTimeout(context.Background(), time.Second)
			defer closeCancel()
			_ = conn.Close(closeCtx)
		}()
	}
	if err != nil {
		return false
	}
	signalWake(out, Listening)
	for {
		notification, err := conn.WaitForNotification(ctx)
		if err != nil {
			return true
		}
		for _, domain := range p.Domains {
			if notification.Payload == string(domain) {
				signalWake(out, Changed)
				break
			}
		}
	}
}
