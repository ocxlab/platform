// provision creates synthetic merchant credentials for local development only.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"ocp.local/platform/internal/persistence"
	"os"
	"time"
)

func main() {
	provider := flag.Bool("provider", false, "Provision a separate local provider administration credential")
	flag.Parse()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	db, err := persistence.Open(ctx, os.Getenv("OCP_MIGRATION_DATABASE_URL"))
	if err != nil {
		fmt.Fprintln(os.Stderr, "administrative database unavailable")
		os.Exit(1)
	}
	defer db.Close()
	if *provider {
		p, token, err := persistence.ProvisionProvider(ctx, db, []string{"publication:read", "publication:manage", "order:inspect"}, time.Now().Add(24*time.Hour))
		if err != nil {
			fmt.Fprintln(os.Stderr, "provider provisioning failed")
			os.Exit(1)
		}
		_ = json.NewEncoder(os.Stdout).Encode(map[string]any{"provider_grant_id": p.ID, "token": token, "expires_in_hours": 24})
		return
	}
	p, token, err := persistence.Provision(ctx, db, "Local example merchant", []string{"store:read", "store:write", "offering:read", "offering:write", "order:read", "order:manage"}, time.Now().Add(24*time.Hour))
	if err != nil {
		fmt.Fprintln(os.Stderr, "provisioning failed")
		os.Exit(1)
	}
	// The sole plaintext disclosure is explicit stdout, not normal API logs.
	_ = json.NewEncoder(os.Stdout).Encode(map[string]any{"merchant_id": p.MerchantID, "token": token, "expires_in_hours": 24})
}
