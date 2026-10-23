// Local account fixture/credential bootstrap; never a production signup endpoint.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/google/uuid"
	"ocp.local/platform/internal/core/persistence"
	"ocp.local/platform/internal/infra/database"
	"os"
	"time"
)

func main() {
	kind := flag.String("kind", "customer", "merchant or customer")
	transport := flag.String("transport", "mcp", "rest or mcp")
	account := flag.String("account", "", "existing account UUID; omit to create a new fixture")
	merchant := flag.String("merchant", "", "owning merchant UUID for a new merchant account")
	name := flag.String("name", "", "personal name for a new account fixture")
	flag.Parse()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	db, err := database.Open(ctx, os.Getenv("OCP_MIGRATION_DATABASE_URL"))
	if err != nil {
		fail()
	}
	defer db.Close()
	var databaseName string
	if db.QueryRowContext(ctx, "SELECT current_database()").Scan(&databaseName) != nil || databaseName != "ocp_dev" {
		fail()
	}
	var id uuid.UUID
	if *account != "" {
		id, err = uuid.Parse(*account)
		if err != nil {
			fail()
		}
	} else {
		mid := uuid.Nil
		if *merchant != "" {
			mid, err = uuid.Parse(*merchant)
			if err != nil {
				fail()
			}
		}
		id, err = persistence.ProvisionAccount(ctx, db, *kind, *name, mid)
		if err != nil {
			fail()
		}
	}
	expiry := time.Now().Add(time.Hour)
	token, err := persistence.ProvisionAccountGrant(ctx, db, id, *kind, *transport, []string{"account:read", "account:write"}, expiry)
	if err != nil {
		fail()
	}
	endpoint := "/mcp/account/" + *kind
	if *transport == "rest" {
		endpoint = "/api/v1/" + *kind + "-account/profile"
	}
	_ = json.NewEncoder(os.Stdout).Encode(map[string]any{"account_id": id, "resource": persistence.AccountResource(*kind, *transport), "token": token, "expires_at": expiry, "endpoint": endpoint})
}
func fail() {
	fmt.Fprintln(os.Stderr, "Local account provisioning failed; check kind, identity, and transport in ocp_dev.")
	os.Exit(1)
}
