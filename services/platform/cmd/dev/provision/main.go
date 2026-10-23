// provision creates synthetic merchant credentials for local development only.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"ocp.local/platform/internal/core/persistence"
	"ocp.local/platform/internal/infra/database"
	"ocp.local/platform/tests/devsetup"
	"os"
	"time"
)

func main() {
	provider := flag.Bool("provider", false, "Provision a separate local provider administration credential")
	list := flag.Bool("list", false, "List local stores and their owning merchants without credentials")
	mcp := flag.Bool("merchant-mcp", false, "Provision a resource-scoped local merchant MCP credential for an existing merchant")
	flag.Parse()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	db, err := database.Open(ctx, os.Getenv("OCP_MIGRATION_DATABASE_URL"))
	if err != nil {
		fmt.Fprintln(os.Stderr, "administrative database unavailable")
		os.Exit(1)
	}
	defer db.Close()
	merchant, store := os.Getenv("OCP_DEV_MERCHANT"), os.Getenv("OCP_DEV_STORE")
	if *mcp && (*provider || *list || (merchant == "" && store == "")) {
		fmt.Fprintln(os.Stderr, "merchant MCP requires an existing merchant/store selector and cannot use provider/list mode")
		os.Exit(1)
	}
	if (*provider || *list) && (merchant != "" || store != "") {
		fmt.Fprintln(os.Stderr, "merchant/store selectors cannot be used for administration credentials or listings")
		os.Exit(1)
	}
	if *list {
		stores, err := devsetup.Stores(ctx, db)
		if err != nil {
			fmt.Fprintln(os.Stderr, "store listing failed")
			os.Exit(1)
		}
		_ = json.NewEncoder(os.Stdout).Encode(stores)
		return
	}
	if *provider {
		p, token, err := persistence.ProvisionProvider(ctx, db, []string{"publication:read", "publication:manage", "order:inspect"}, time.Now().Add(24*time.Hour))
		if err != nil {
			fmt.Fprintln(os.Stderr, "provider provisioning failed")
			os.Exit(1)
		}
		_ = json.NewEncoder(os.Stdout).Encode(map[string]any{"provider_grant_id": p.ID, "token": token, "expires_in_hours": 24})
		return
	}
	scopes := []string{"store:read", "store:write", "offering:read", "offering:write", "order:read", "order:manage"}
	var p persistence.Principal
	var token string
	if merchant != "" || store != "" {
		id, resolveErr := devsetup.ResolveMerchant(ctx, db, merchant, store)
		if resolveErr != nil {
			fmt.Fprintln(os.Stderr, "merchant selection failed:", resolveErr)
			os.Exit(1)
		}
		if *mcp {
			scopes = []string{"store:read", "offering:read", "offering:write"}
			p, token, err = persistence.ProvisionMerchantMCPGrant(ctx, db, id, scopes, time.Now().Add(time.Hour))
		} else {
			p, token, err = persistence.ProvisionMerchantGrant(ctx, db, id, scopes, time.Now().Add(24*time.Hour))
		}
	} else {
		fmt.Fprintln(os.Stderr, "Creating a NEW merchant. Use STORE=store-N or MERCHANT=<UUID> to access an existing merchant.")
		p, token, err = persistence.Provision(ctx, db, "Local example merchant", scopes, time.Now().Add(24*time.Hour))
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "provisioning failed")
		os.Exit(1)
	}
	// The sole plaintext disclosure is explicit stdout, not normal API logs.
	expiry := 24
	resource := persistence.MerchantREST
	if *mcp {
		expiry = 1
		resource = persistence.MerchantMCP
	}
	_ = json.NewEncoder(os.Stdout).Encode(map[string]any{"merchant_id": p.MerchantID, "token": token, "expires_in_hours": expiry, "resource": resource})
}
