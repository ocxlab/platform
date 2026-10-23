package work

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"ocp.local/platform/internal/core/capability"
	"ocp.local/platform/internal/infra/telemetry"
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
	DSN      string
	Domains  []capability.Domain
	Observer *telemetry.Observer
}

func signalWake(out chan<- WakeSignal, signal WakeSignal) bool {
	select {
	case out <- signal:
		return true
	default:
		return false
	}
}

func reconnectDelay(previous time.Duration, stable bool) time.Duration {
	if stable {
		return time.Second
	}
	return max(time.Second, min(30*time.Second, previous*2))
}

func (p PostgresWake) Run(ctx context.Context, out chan<- WakeSignal) error {
	backoff := time.Duration(0)
	for ctx.Err() == nil {
		started := time.Now()
		connected := p.listen(ctx, out)
		// A successful LISTEN that immediately disconnects is still a failure
		// streak. Reset only after a minimally stable session.
		backoff = reconnectDelay(backoff, connected && time.Since(started) >= 30*time.Second)
		timer := time.NewTimer(backoff)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil
		case <-timer.C:
		}
	}
	return nil
}

func (p PostgresWake) listen(ctx context.Context, out chan<- WakeSignal) bool {
	p.Observer.WorkEvent(ctx, "listen_attempt")
	connect, cancel := context.WithTimeout(ctx, 5*time.Second)
	config, err := pgx.ParseConfig(p.DSN)
	if err != nil {
		cancel()
		p.Observer.WorkEvent(ctx, "listen_failure")
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
		p.Observer.WorkEvent(ctx, "listen_failure")
		return false
	}
	p.Observer.WorkEvent(ctx, "listening")
	if !signalWake(out, Listening) {
		p.Observer.WorkEvent(ctx, "coalesced")
	}
	for {
		notification, err := conn.WaitForNotification(ctx)
		if err != nil {
			if ctx.Err() == nil {
				p.Observer.WorkEvent(ctx, "disconnected")
			}
			return true
		}
		matched := false
		for _, domain := range p.Domains {
			if notification.Payload == string(domain) {
				matched = true
				p.Observer.WorkEvent(ctx, "notification")
				if !signalWake(out, Changed) {
					p.Observer.WorkEvent(ctx, "coalesced")
				}
				break
			}
		}
		if !matched {
			p.Observer.WorkEvent(ctx, "ignored_notification")
		}
	}
}
