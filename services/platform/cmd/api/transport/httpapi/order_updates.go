package httpapi

import (
	"context"
	"fmt"
	"net/http"
	foodstore "ocp.local/platform/internal/food/persistence"
	"strings"
	"time"

	"github.com/google/uuid"
	"ocp.local/platform/internal/core/persistence"
	"ocp.local/platform/internal/core/updates"
)

func (s *Server) WatchGuestFoodOrder(w http.ResponseWriter, r *http.Request, merchant, store, order uuid.UUID) {
	s.watchOrders(w, r, orderupdates.Change{Merchant: merchant, Store: store, Order: order})
}
func (s *Server) WatchMerchantFoodOrders(w http.ResponseWriter, r *http.Request, merchant, store uuid.UUID) {
	s.watchOrders(w, r, orderupdates.Change{Merchant: merchant, Store: store})
}
func (s *Server) watchOrders(w http.ResponseWriter, r *http.Request, change orderupdates.Change) {
	// Subscribe before the initial read to close the read/subscribe race.
	if s.updates == nil {
		problem(w, 503, "unavailable", "Live updates are temporarily unavailable.", nil)
		return
	}
	hints, unsubscribe, ok := s.updates.Subscribe(change, principal(r).ID)
	if !ok {
		problem(w, 503, "unavailable", "Live updates are temporarily unavailable.", nil)
		return
	}
	defer unsubscribe()
	authorize := func(read bool) error {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		p := principal(r)
		authenticate := persistence.Authenticate
		if p.Guest {
			authenticate = persistence.AuthenticateGuest
		}
		current, err := authenticate(ctx, s.db, strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
		if err != nil {
			return err
		}
		if !current.Allows("order:read") || current.MerchantID != change.Merchant || (current.Guest && current.StoreID != change.Store) {
			return persistence.ErrUnauthenticated
		}
		if !read {
			return nil
		}
		if p.Guest {
			_, err = foodstore.GetGuestFoodOrder(ctx, s.db, current, change.Order)
		} else {
			_, err = foodstore.ListFoodOrders(ctx, s.db, current, change.Store, nil, 1)
		}
		return err
	}
	if err := authorize(true); err != nil {
		write(w, nil, err)
		return
	}
	controller := http.NewResponseController(w)
	send := func(frame string) bool {
		if controller.SetWriteDeadline(time.Now().Add(15*time.Second)) != nil {
			return false
		}
		if _, err := fmt.Fprint(w, frame); err != nil {
			return false
		}
		return controller.Flush() == nil
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("X-Accel-Buffering", "no")
	if !send("retry: 3000\nevent: invalidate\ndata: {\"resource\":\"food_orders\"}\n\n") {
		return
	}
	// The write deadline is renewed by each short heartbeat, overriding only the
	// stream's normal response deadline. No transaction or DB connection is held.
	heartbeat := time.NewTicker(10 * time.Second)
	defer heartbeat.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case _, open := <-hints:
			if !open || authorize(true) != nil {
				return
			}
			if !send("event: invalidate\ndata: {\"resource\":\"food_orders\"}\n\n") {
				return
			}
		case <-heartbeat.C:
			if authorize(false) != nil || !send(": heartbeat\n\n") {
				return
			}
		}
	}
}
