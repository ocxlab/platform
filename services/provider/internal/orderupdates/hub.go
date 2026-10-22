// Package orderupdates delivers disposable change hints, never commerce records.
package orderupdates

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type Change struct{ Merchant, Store, Order uuid.UUID }
type subscription struct {
	change Change
	signal chan struct{}
	actor  uuid.UUID
}
type Hub struct {
	mu            sync.Mutex
	ready         bool
	subscriptions map[*subscription]struct{}
}

func New() *Hub { return &Hub{subscriptions: make(map[*subscription]struct{})} }

// Subscribe fails closed while disconnected. Reconnection closes existing streams;
// every new stream requests an authoritative fresh read instead of claiming replay.
func (h *Hub) Subscribe(change Change, actor uuid.UUID) (<-chan struct{}, func(), bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if !h.ready || len(h.subscriptions) >= 1000 {
		return nil, nil, false
	}
	count := 0
	for s := range h.subscriptions {
		if s.actor == actor {
			count++
		}
	}
	if count >= 8 {
		return nil, nil, false
	}
	s := &subscription{change, make(chan struct{}, 1), actor}
	h.subscriptions[s] = struct{}{}
	return s.signal, func() { h.mu.Lock(); delete(h.subscriptions, s); h.mu.Unlock() }, true
}
func (h *Hub) disconnect() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.ready = false
	for s := range h.subscriptions {
		close(s.signal)
		delete(h.subscriptions, s)
	}
}
func (h *Hub) publish(change Change) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for s := range h.subscriptions {
		if s.change.Merchant == change.Merchant && s.change.Store == change.Store && (s.change.Order == uuid.Nil || s.change.Order == change.Order) {
			select {
			case s.signal <- struct{}{}:
			default:
			} // coalesce; never block a writer
		}
	}
}

// Run reserves one direct database connection per API process, not per browser.
// There is no order-data polling. A reconnect forces client resynchronization.
func (h *Hub) Run(ctx context.Context, dsn string) {
	defer h.disconnect()
	for ctx.Err() == nil {
		h.listen(ctx, dsn)
		h.disconnect()
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Second):
		}
	}
}
func (h *Hub) listen(ctx context.Context, dsn string) {
	connect, cancel := context.WithTimeout(ctx, 5*time.Second)
	conn, err := pgx.Connect(connect, dsn)
	if err == nil {
		_, err = conn.Exec(connect, "LISTEN ocp_food_order_changed")
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
		return
	}
	h.mu.Lock()
	h.ready = true
	h.mu.Unlock()
	for {
		notification, err := conn.WaitForNotification(ctx)
		if err != nil {
			return
		}
		var change Change
		if json.Unmarshal([]byte(notification.Payload), &change) == nil && change.Merchant != uuid.Nil && change.Store != uuid.Nil && change.Order != uuid.Nil {
			h.publish(change)
		}
	}
}
