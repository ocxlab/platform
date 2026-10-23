package httpapi

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/google/uuid"
	"ocp.local/platform/internal/core/persistence"
	"ocp.local/platform/internal/orderupdates"
)

func (s *Server) WatchPublicOffering(w http.ResponseWriter, r *http.Request, merchant, store uuid.UUID) {
	if s.offeringUpdates == nil {
		problem(w, 503, "unavailable", "Live updates are temporarily unavailable.", nil)
		return
	}
	// Bound total streams and streams per direct peer. Forwarded headers are not
	// trusted. A production reverse proxy needs an explicit trusted-peer policy.
	peer, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		peer = r.RemoteAddr
	}
	actor := uuid.NewSHA1(uuid.NameSpaceOID, []byte(peer))
	hints, unsubscribe, ok := s.offeringUpdates.Subscribe(orderupdates.Change{Merchant: merchant, Store: store}, actor)
	if !ok {
		problem(w, 503, "unavailable", "Live updates are temporarily unavailable.", nil)
		return
	}
	defer unsubscribe()
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	_, err = persistence.GetPublicOffering(ctx, s.db, merchant, store)
	cancel()
	if err != nil {
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
	const hint = "event: invalidate\ndata: {\"resource\":\"public_offering\"}\n\n"
	if !send("retry: 3000\n" + hint) {
		return
	}
	heartbeat := time.NewTicker(10 * time.Second)
	defer heartbeat.Stop()
	lifetime := time.NewTimer(2 * time.Minute)
	defer lifetime.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-lifetime.C:
			return
		case _, open := <-hints:
			if !open || !send(hint) {
				return
			}
			// Notify existing viewers of withdrawal, then stop. Do not let a
			// formerly public subscription observe subsequent private activity.
			readCtx, readCancel := context.WithTimeout(r.Context(), 5*time.Second)
			_, err := persistence.GetPublicOffering(readCtx, s.db, merchant, store)
			readCancel()
			if err != nil {
				return
			}
		case <-heartbeat.C:
			if !send(": heartbeat\n\n") {
				return
			}
		}
	}
}
