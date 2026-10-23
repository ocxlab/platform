// seed-demo is deliberately hard-wired to the local Compose development database.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"ocp.local/platform/internal/infra/database"
	"ocp.local/platform/tests/devsetup"
	"os"
	"time"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	db, err := database.Open(ctx, "postgres://ocp_dev:ocp_local_only@db:5432/ocp_dev?sslmode=disable")
	if err != nil {
		fail(err)
	}
	defer db.Close()
	store, err := devsetup.EnsureRestaurant(ctx, db)
	if err != nil {
		fail(err)
	}
	_ = json.NewEncoder(os.Stdout).Encode(map[string]any{"store": store, "login_command": "task dev:credential STORE=" + store.Handle, "note": "Fictional demo; existing edits and publication are preserved on repeat runs."})
}
func fail(err error) { fmt.Fprintln(os.Stderr, "demo seed failed:", err); os.Exit(1) }
