// provision creates synthetic merchant credentials for local development only.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"ocp.local/platform/internal/persistence"
	"os"
	"time"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	db, err := persistence.Open(ctx, os.Getenv("OCP_MIGRATION_DATABASE_URL"))
	if err != nil {
		fmt.Fprintln(os.Stderr, "administrative database unavailable")
		os.Exit(1)
	}
	defer db.Close()
	p, token, err := persistence.Provision(ctx, db, "Local example merchant", []string{"store:read", "store:write", "offering:read", "offering:write"}, time.Now().Add(24*time.Hour))
	if err != nil {
		fmt.Fprintln(os.Stderr, "provisioning failed")
		os.Exit(1)
	}
	// The sole plaintext disclosure is explicit stdout, not normal API logs.
	_ = json.NewEncoder(os.Stdout).Encode(map[string]any{"merchant_id": p.MerchantID, "token": token, "expires_in_hours": 24})
}
