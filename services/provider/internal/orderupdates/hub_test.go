package orderupdates

import (
	"testing"

	"github.com/google/uuid"
)

func TestScopeCoalescingDisconnectAndCapacity(t *testing.T) {
	h := New()
	change := Change{uuid.New(), uuid.New(), uuid.New()}
	actor := uuid.New()
	if _, _, ok := h.Subscribe(change, actor); ok {
		t.Fatal("disconnected listener accepted subscription")
	}
	h.ready = true
	order, releaseOrder, ok := h.Subscribe(change, actor)
	if !ok {
		t.Fatal("subscribe")
	}
	defer releaseOrder()
	store, releaseStore, _ := h.Subscribe(Change{Merchant: change.Merchant, Store: change.Store}, actor)
	defer releaseStore()
	unrelated, releaseOther, _ := h.Subscribe(Change{uuid.New(), change.Store, change.Order}, actor)
	defer releaseOther()
	h.publish(change)
	h.publish(change)
	for _, signal := range []<-chan struct{}{order, store} {
		select {
		case <-signal:
		default:
			t.Fatal("missing hint")
		}
		select {
		case <-signal:
			t.Fatal("duplicate hints not coalesced")
		default:
		}
	}
	select {
	case <-unrelated:
		t.Fatal("cross-merchant hint")
	default:
	}
	h.publish(Change{change.Merchant, uuid.New(), change.Order})
	select {
	case <-order:
		t.Fatal("cross-store hint")
	default:
	}
	h.publish(Change{change.Merchant, change.Store, uuid.New()})
	select {
	case <-order:
		t.Fatal("different-order hint")
	default:
	}
	select {
	case <-store:
	default:
		t.Fatal("store inbox missed other order")
	}
	for i := 0; i < 5; i++ {
		_, release, ok := h.Subscribe(change, actor)
		if !ok {
			t.Fatal("early limit")
		}
		defer release()
	}
	if _, _, ok := h.Subscribe(change, actor); ok {
		t.Fatal("per-credential limit missing")
	}
	if _, release, ok := h.Subscribe(change, uuid.New()); !ok {
		t.Fatal("limit leaked to other credential")
	} else {
		release()
	}
	h.disconnect()
	if _, open := <-order; open {
		t.Fatal("disconnected channel not closed")
	}
	if _, _, ok := h.Subscribe(change, actor); ok {
		t.Fatal("subscribe while disconnected")
	}
}
