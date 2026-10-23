// Local bootstrap only; this is not an OAuth token endpoint.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	core "ocp.local/platform/internal/core/persistence"
	food "ocp.local/platform/internal/food/persistence"
	"ocp.local/platform/internal/infra/database"
	"os"
	"time"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	db, err := database.Open(ctx, os.Getenv("OCP_MIGRATION_DATABASE_URL"))
	if err != nil {
		fail()
	}
	defer db.Close()
	var name string
	if db.QueryRowContext(ctx, "SELECT current_database()").Scan(&name) != nil || name != "ocp_dev" {
		fail()
	}
	shop, err := core.GetPublicStoreByHandle(ctx, db, os.Getenv("OCP_DEV_STORE"))
	if err != nil {
		fail()
	}
	session, err := food.CreateGuestMCPSession(ctx, db, shop.MerchantID, shop.ID)
	if err != nil {
		fail()
	}
	_ = json.NewEncoder(os.Stdout).Encode(map[string]any{"store": shop.Handle, "token": session.Token, "expires_at": session.ExpiresAt, "resource": "customer-food-mcp", "endpoint": "/mcp/customer/food"})
}
func fail() {
	fmt.Fprintln(os.Stderr, "Local customer MCP provisioning failed; select a published STORE=store-N in ocp_dev.")
	os.Exit(1)
}
