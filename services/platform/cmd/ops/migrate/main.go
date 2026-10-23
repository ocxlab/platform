// migrate is an explicit administrative command, never run by API startup.
package main

import (
	"context"
	"fmt"
	"ocp.local/platform/internal/infra/database"
	"os"
	"time"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	db, err := database.Open(ctx, os.Getenv("OCP_MIGRATION_DATABASE_URL"))
	if err == nil {
		defer db.Close()
		err = database.Migrate(ctx, db)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "migration failed:", err)
		os.Exit(1)
	}
	fmt.Println("Migrations applied; no application data reset")
}
