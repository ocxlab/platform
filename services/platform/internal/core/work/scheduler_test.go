package work

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"ocp.local/platform/internal/core/capability"
	"ocp.local/platform/internal/core/workmodel"
	"ocp.local/platform/internal/infra/telemetry"
)

type fakeStore struct {
	mu            sync.Mutex
	claims        int
	schedules     int
	remaining     int
	claimTimes    []time.Time
	delay         *time.Duration
	finish        func(context.Context, workmodel.Effect, bool, time.Duration) (bool, error)
	claimError    error
	scheduleError error
}

func (s *fakeStore) Claim(ctx context.Context, domains []capability.Domain, batch int, lease time.Duration) ([]workmodel.Effect, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.claims++
	s.claimTimes = append(s.claimTimes, time.Now())
	if s.claimError != nil {
		return nil, s.claimError
	}
	var effects []workmodel.Effect
	for range min(batch, s.remaining) {
		effects = append(effects, workmodel.Effect{ID: uuid.New(), Domain: capability.Food, Kind: "catalog.publish", Attempts: 1})
	}
	s.remaining -= len(effects)
	return effects, nil
}
func (s *fakeStore) Finish(ctx context.Context, e workmodel.Effect, success bool, retry time.Duration) (bool, error) {
	if s.finish != nil {
		return s.finish(ctx, e, success, retry)
	}
	return true, nil
}
func (s *fakeStore) NextDelay(context.Context, []capability.Domain) (*time.Duration, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.schedules++
	return s.delay, s.scheduleError
}
func (s *fakeStore) counts() (int, int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.claims, s.schedules
}

type fakeWake struct{ input chan WakeSignal }

func (f fakeWake) Run(ctx context.Context, out chan<- WakeSignal) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case signal := <-f.input:
			signalWake(out, signal)
		}
	}
}
func testWorker(store Store, wake WakeSource) Worker {
	return Worker{Store: store, Wake: wake, Domains: []capability.Domain{capability.Food}, Concurrency: 2, Handlers: map[string]Handler{"food/catalog.publish": func(context.Context, workmodel.Effect) error { return nil }}}
}
func waitFor(t *testing.T, condition func() bool) {
	t.Helper()
	until := time.Now().Add(2 * time.Second)
	for time.Now().Before(until) {
		if condition() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("condition timed out")
}
func TestNextWait(t *testing.T) {
	for _, tc := range []struct{ due, want time.Duration }{{-time.Hour, minDrainInterval}, {0, minDrainInterval}, {time.Second, time.Second}, {time.Hour, time.Minute}} {
		if got := nextWait(&tc.due, time.Minute); got != tc.want {
			t.Errorf("%s: got %s want %s", tc.due, got, tc.want)
		}
	}
	if nextWait(nil, time.Minute) != time.Minute {
		t.Fatal("lost safety scan")
	}
}
func TestSchedulerListenBeforeScanAndCoalescesHints(t *testing.T) {
	store := &fakeStore{}
	wake := fakeWake{make(chan WakeSignal, 2000)}
	w := testWorker(store, wake)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- w.Run(ctx) }()
	time.Sleep(30 * time.Millisecond)
	if claims, _ := store.counts(); claims != 0 {
		t.Fatal("scanned before listener readiness")
	}
	wake.input <- Listening
	waitFor(t, func() bool { _, s := store.counts(); return s == 1 })
	for range 1000 {
		wake.input <- Changed
	}
	waitFor(t, func() bool { _, s := store.counts(); return s >= 2 })
	time.Sleep(150 * time.Millisecond)
	claims, _ := store.counts()
	if claims > 4 {
		t.Fatalf("query per hint: %d", claims)
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestSchedulerSpacesRepeatedDrains(t *testing.T) {
	store := &fakeStore{}
	wake := fakeWake{make(chan WakeSignal, 1)}
	w := testWorker(store, wake)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- w.Run(ctx) }()
	for n := 1; n <= 3; n++ {
		wake.input <- Changed
		waitFor(t, func() bool { c, _ := store.counts(); return c >= n })
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	for n := 1; n < len(store.claimTimes); n++ {
		if elapsed := store.claimTimes[n].Sub(store.claimTimes[n-1]); elapsed < 90*time.Millisecond {
			t.Fatalf("wake hints bypassed drain spacing: %s", elapsed)
		}
	}
}
func TestSchedulerSafetyAndDueTimersWithoutHints(t *testing.T) {
	for _, safety := range []bool{true, false} {
		t.Run(map[bool]string{true: "safety", false: "due"}[safety], func(t *testing.T) {
			store := &fakeStore{}
			wake := fakeWake{make(chan WakeSignal, 1)}
			w := testWorker(store, wake)
			if safety {
				w.SafetyInterval = 100 * time.Millisecond
			} else {
				d := 150 * time.Millisecond
				store.delay = &d
				wake.input <- Listening
			}
			ctx, cancel := context.WithCancel(context.Background())
			done := make(chan error, 1)
			go func() { done <- w.Run(ctx) }()
			defer cancel()
			waitFor(t, func() bool { c, _ := store.counts(); return c >= 2 })
			cancel()
			if err := <-done; err != nil {
				t.Fatal(err)
			}
		})
	}
}
func TestSchedulerBoundsBacklogAndStopsClaiming(t *testing.T) {
	d := time.Duration(0)
	store := &fakeStore{remaining: 100, delay: &d}
	wake := fakeWake{make(chan WakeSignal, 1)}
	wake.input <- Listening
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	w := testWorker(store, wake)
	go func() { done <- w.Run(ctx) }()
	waitFor(t, func() bool { _, s := store.counts(); return s >= 1 })
	claims, _ := store.counts()
	if claims != 8 {
		t.Fatalf("unbounded drain: %d", claims)
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	before, _ := store.counts()
	time.Sleep(120 * time.Millisecond)
	after, _ := store.counts()
	if before != after {
		t.Fatal("claimed during shutdown")
	}
}
func TestOnceFinishesWithBoundedUncancelledCleanup(t *testing.T) {
	store := &fakeStore{remaining: 1}
	started := make(chan struct{})
	finished := make(chan bool, 1)
	store.finish = func(ctx context.Context, e workmodel.Effect, success bool, retry time.Duration) (bool, error) {
		_, bounded := ctx.Deadline()
		finished <- ctx.Err() == nil && bounded && !success && retry == 2*time.Second && e.ID != uuid.Nil
		return true, nil
	}
	w := testWorker(store, nil)
	w.Handlers["food/catalog.publish"] = func(ctx context.Context, e workmodel.Effect) error { close(started); <-ctx.Done(); return ctx.Err() }
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { _, err := w.Once(ctx); done <- err }()
	<-started
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if !<-finished {
		t.Fatal("cleanup lost cancellation boundary or stable identity")
	}
}
func TestSchedulerRejectsBadConfiguration(t *testing.T) {
	w := testWorker(&fakeStore{}, fakeWake{make(chan WakeSignal)})
	w.Concurrency = 0
	if err := w.Run(context.Background()); err == nil {
		t.Fatal("accepted invalid concurrency")
	}
	w.Concurrency = 2
	w.SafetyInterval = -1
	if err := w.Run(context.Background()); err == nil {
		t.Fatal("accepted invalid safety interval")
	}
}

func TestSchedulerAcceptsMaximumSafetyInterval(t *testing.T) {
	w := testWorker(&fakeStore{}, fakeWake{make(chan WakeSignal)})
	w.SafetyInterval = time.Hour
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := w.Run(ctx); err != nil {
		t.Fatalf("rejected supported safety boundary: %v", err)
	}
}
func TestSchedulerRecoversStoreError(t *testing.T) {
	store := &fakeStore{claimError: errors.New("temporary database failure")}
	wake := fakeWake{make(chan WakeSignal, 1)}
	wake.input <- Listening
	w := testWorker(store, wake)
	w.SafetyInterval = 100 * time.Millisecond
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- w.Run(ctx) }()
	waitFor(t, func() bool { c, _ := store.counts(); return c >= 1 })
	store.mu.Lock()
	store.claimError = nil
	store.mu.Unlock()
	waitFor(t, func() bool { _, s := store.counts(); return s >= 1 })
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

type stoppedWake struct{}

type silentlyStoppedWake struct{}

func (silentlyStoppedWake) Run(context.Context, chan<- WakeSignal) error { return nil }

func TestUnexpectedCleanWakeExitIsAnError(t *testing.T) {
	w := testWorker(&fakeStore{}, silentlyStoppedWake{})
	if err := w.Run(context.Background()); err == nil || err.Error() != "work wake source stopped" {
		t.Fatalf("unexpected clean exit was hidden: %v", err)
	}
}

func TestReconnectDelay(t *testing.T) {
	previous := time.Duration(0)
	for _, want := range []time.Duration{time.Second, 2 * time.Second, 4 * time.Second, 8 * time.Second, 16 * time.Second, 30 * time.Second, 30 * time.Second} {
		previous = reconnectDelay(previous, false)
		if previous != want {
			t.Fatalf("flapping connection delay: got %s want %s", previous, want)
		}
	}
	if got := reconnectDelay(previous, true); got != time.Second {
		t.Fatalf("stable connection did not reset delay: %s", got)
	}
}

func (stoppedWake) Run(context.Context, chan<- WakeSignal) error { return errors.New("stopped") }
func TestUnexpectedWakeExitStopsWorker(t *testing.T) {
	w := testWorker(&fakeStore{}, stoppedWake{})
	if err := w.Run(context.Background()); err == nil {
		t.Fatal("wake exit hidden")
	}
}
func TestWakeCancellationInterruptsReconnectBackoff(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- (PostgresWake{DSN: "not a valid DSN"}).Run(ctx, make(chan WakeSignal, 1)) }()
	time.Sleep(20 * time.Millisecond)
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(400 * time.Millisecond):
		t.Fatal("backoff blocked cancellation")
	}
}
func TestSchedulerRecoversScheduleError(t *testing.T) {
	store := &fakeStore{scheduleError: errors.New("temporary schedule failure")}
	wake := fakeWake{make(chan WakeSignal, 1)}
	wake.input <- Listening
	w := testWorker(store, wake)
	w.SafetyInterval = 100 * time.Millisecond
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- w.Run(ctx) }()
	waitFor(t, func() bool { _, s := store.counts(); return s >= 1 })
	store.mu.Lock()
	store.scheduleError = nil
	store.mu.Unlock()
	waitFor(t, func() bool { _, s := store.counts(); return s >= 2 })
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

type failingStats struct{ calls atomic.Int32 }

func (s *failingStats) Snapshot(context.Context, []capability.Domain) ([]workmodel.Stats, error) {
	s.calls.Add(1)
	return nil, errors.New("private-storage-error")
}
func TestStatsFailureIsOptionalAndRateLimited(t *testing.T) {
	runtime, err := telemetry.New(context.Background(), telemetry.Config{Exporter: "none"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := runtime.Shutdown(context.Background()); err != nil {
			t.Error(err)
		}
	}()
	stats := &failingStats{}
	store := &fakeStore{remaining: 1}
	wake := fakeWake{make(chan WakeSignal, 1)}
	wake.input <- Listening
	w := testWorker(store, wake)
	w.SafetyInterval = 100 * time.Millisecond
	w.Observer = runtime.Observer
	w.Stats = stats
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- w.Run(ctx) }()
	waitFor(t, func() bool { _, s := store.counts(); return s >= 3 })
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if stats.calls.Load() != 1 {
		t.Fatalf("snapshot queried per drain: %d", stats.calls.Load())
	}
	store.mu.Lock()
	remaining := store.remaining
	store.mu.Unlock()
	if remaining != 0 {
		t.Fatal("snapshot failure blocked durable work")
	}
}
