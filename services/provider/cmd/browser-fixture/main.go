// browser-fixture provisions synthetic data ONLY into the disposable test service.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"ocp.local/platform/internal/devfixtures"
	"ocp.local/platform/internal/persistence"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	// No configurable production DSN: this command targets the dedicated Compose test DB.
	db, err := persistence.Open(ctx, "postgres://ocp_test:ocp_test_local_only@test-db:5432/ocp_test?sslmode=disable")
	if err != nil {
		fail()
	}
	defer db.Close()
	if persistence.Migrate(ctx, db) != nil {
		fail()
	}
	p, token, err := persistence.Provision(ctx, db, "Browser fixture merchant", []string{"store:read", "store:write", "offering:read", "offering:write", "order:read", "order:manage"}, time.Now().Add(time.Hour))
	if err != nil {
		fail()
	}
	_, provider, err := persistence.ProvisionProvider(ctx, db, []string{"publication:read", "publication:manage", "order:inspect"}, time.Now().Add(time.Hour))
	if err != nil {
		fail()
	}
	additional, additionalToken, err := persistence.Provision(ctx, db, "Extras browser merchant", []string{"store:read", "store:write", "offering:read", "offering:write", "order:read", "order:manage"}, time.Now().Add(time.Hour))
	if err != nil {
		fail()
	}
	// stdout is piped directly to the browser runner, never a tracked fixture or log.
	demo, demoToken, err := persistence.Provision(ctx, db, "Large menu browser merchant", []string{"store:read", "store:write", "offering:read", "offering:write", "order:read", "order:manage"}, time.Now().Add(time.Hour))
	if err != nil {
		fail()
	}
	menu, _, err := devfixtures.Restaurant()
	if err != nil {
		fail()
	}
	rawMenu, err := json.Marshal(menu)
	if err != nil {
		fail()
	}
	if json.NewEncoder(os.Stdout).Encode(map[string]string{"merchant_id": p.MerchantID.String(), "merchant_token": token, "provider_token": provider, "extras_merchant_id": additional.MerchantID.String(), "extras_merchant_token": additionalToken, "demo_merchant_id": demo.MerchantID.String(), "demo_merchant_token": demoToken, "demo_menu": string(rawMenu)}) != nil {
		fail()
	}
}
func fail() { fmt.Fprintln(os.Stderr, "disposable browser fixture preparation failed"); os.Exit(1) }
