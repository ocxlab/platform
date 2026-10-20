package main

import (
	"context"
	"testing"
	"time"
)

func TestRunExitsOnCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	done := make(chan error, 1)
	go func() { done <- run(ctx, "127.0.0.1:0") }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("shutdown: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("API did not stop after cancellation")
	}
}

func TestRunReportsListenFailure(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := run(ctx, "not-an-address"); err == nil {
		t.Fatal("invalid listen address should fail")
	}
}
