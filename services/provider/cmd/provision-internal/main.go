// provision-internal explicitly prints one temporary local cohort credential.
// It is an administrative CLI, not an HTTP authentication endpoint.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/google/uuid"
	"ocp.local/platform/internal/capability"
	"ocp.local/platform/internal/persistence"
	"ocp.local/platform/internal/sessionkit"
	"ocp.local/platform/internal/ucpinternal"
	"os"
	"time"
)

func main() {
	domain := flag.String("domain", "shopping", "shopping or lodging")
	management := flag.Bool("management", false, "issue catalog management authority instead of customer session authority")
	flag.Parse()
	d := capability.Domain(*domain)
	if d != capability.Shopping && d != capability.Lodging {
		fail("invalid internal domain")
	}
	selector := os.Getenv("OCP_DEV_STORE")
	if selector == "" {
		fail("STORE must select an existing store handle or UUID")
	}
	customer := uuid.New()
	if raw := os.Getenv("OCP_DEV_CUSTOMER"); raw != "" {
		var err error
		customer, err = uuid.Parse(raw)
		if err != nil || customer == uuid.Nil {
			fail("CUSTOMER must be a nonzero UUID")
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	db, err := persistence.Open(ctx, os.Getenv("OCP_MIGRATION_DATABASE_URL"))
	if err != nil {
		fail("administrative database unavailable")
	}
	defer db.Close()
	p := sessionkit.Principal{Domain: d, CustomerID: customer, Profile: ucpinternal.ControlledProfileURI}
	err = db.QueryRowContext(ctx, `SELECT merchant_id,id FROM commerce.stores WHERE handle=$1 OR id::text=$1`, selector).Scan(&p.MerchantID, &p.StoreID)
	if err != nil {
		fail("store selection failed")
	}
	expires := time.Now().Add(time.Hour)
	p, token, err := ucpinternal.Provision(ctx, db, p, ucpinternal.ControlledProfile(d), *management, expires)
	if err != nil {
		fail("internal credential provisioning failed")
	}
	_ = json.NewEncoder(os.Stdout).Encode(map[string]any{"merchant_id": p.MerchantID, "store_id": p.StoreID, "customer_id": p.CustomerID, "domain": d, "management": *management, "profile": p.Profile, "token": token, "expires_at": expires})
}
func fail(message string) { fmt.Fprintln(os.Stderr, message); os.Exit(1) }
