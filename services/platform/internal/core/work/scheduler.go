package work

import (
	"context"
	"errors"
	"time"
)

const minDrainInterval = 100 * time.Millisecond

// nextWait bounds an overdue/locked row's retry cadence and always preserves a
// safety scan. Delay is relative to database time, not the application's clock.
func nextWait(due *time.Duration, safety time.Duration) time.Duration {
	if due == nil {
		return safety
	}
	return min(safety, max(minDrainInterval, *due))
}

func (w Worker) Run(ctx context.Context) error {
	safety := w.SafetyInterval
	if safety == 0 {
		safety = time.Minute
	}
	if w.Store == nil || w.Wake == nil || safety < minDrainInterval || safety > time.Hour {
		return errors.New("invalid work scheduler configuration")
	}
	if err := w.validate(); err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	wakes := make(chan WakeSignal, 1)
	listenerDone := make(chan error, 1)
	go func() { listenerDone <- w.Wake.Run(ctx, wakes) }()
	listenerStopped := false
	defer func() {
		cancel()
		if !listenerStopped {
			select {
			case listenerErr := <-listenerDone:
				// Shutdown has already canceled the wake source.
				_ = listenerErr
			case <-time.After(2 * time.Second):
			}
		}
	}()
	// A live listener triggers the first scan. If it cannot connect at all,
	// the safety timer still recovers work instead of making hints mandatory.
	timer := time.NewTimer(safety)
	defer timer.Stop()
	lastDrain := time.Time{}
	lastStats := time.Time{}
	defer w.Observer.WorkEvent(context.WithoutCancel(ctx), "shutdown")
	for {
		select {
		case <-ctx.Done():
			return nil
		case err := <-listenerDone:
			w.Observer.WorkEvent(ctx, "wake_stopped")
			listenerStopped = true
			if ctx.Err() != nil {
				return nil
			}
			if err == nil {
				err = errors.New("work wake source stopped")
			}
			return err
		case <-wakes:
			w.Observer.WorkEvent(ctx, "drain_wake")
		case <-timer.C:
			w.Observer.WorkEvent(ctx, "drain_timer")
		}
		// Coalesce notification storms without issuing a query per hint.
		if delay := time.Until(lastDrain.Add(minDrainInterval)); delay > 0 {
			pause := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				pause.Stop()
				return nil
			case <-pause.C:
			}
		}
		if ctx.Err() != nil {
			return nil
		}
		lastDrain = time.Now()
		if w.Stats != nil && w.Observer != nil && time.Since(lastStats) >= time.Minute {
			lastStats = time.Now()
			w.recordStats(ctx)
		}
		// Never monopolize a worker loop with an unlimited backlog.
		var drainErr error
		for range 8 {
			count, err := w.Once(ctx)
			if ctx.Err() != nil {
				return nil
			}
			if err != nil {
				drainErr = err
				break
			}
			if count < w.Concurrency {
				break
			}
		}
		if drainErr != nil {
			timer.Reset(min(safety, 5*time.Second))
			continue
		}
		due, err := w.Store.NextDelay(ctx, w.Domains)
		if ctx.Err() != nil {
			return nil
		}
		if err != nil {
			w.Observer.WorkEvent(ctx, "schedule_failure")
			timer.Reset(min(safety, 5*time.Second))
			continue
		}
		timer.Reset(nextWait(due, safety))
	}
}
