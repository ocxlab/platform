// migration-replay applies a supplied directory using the same Goose library as
// the product. It is used only by the isolated migration review tooling.
package main

import (
	"context"
	"database/sql"
	"log"
	"os"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

func main() {
	if len(os.Args) != 2 {
		log.Fatal("usage: migration-replay DIRECTORY (MIGRATION_REPLAY_URL required)")
	}
	db, err := sql.Open("pgx", os.Getenv("MIGRATION_REPLAY_URL"))
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	provider, err := goose.NewProvider(goose.DialectPostgres, db, os.DirFS(os.Args[1]))
	if err != nil {
		log.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if _, err = provider.Up(ctx); err != nil {
		log.Fatal(err)
	}
}
