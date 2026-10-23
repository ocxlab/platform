// Package database owns connection budgets, tenant transactions, and migration execution.
package database

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"io/fs"
	"strconv"
	"time"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var migrations embed.FS

func Open(ctx context.Context, dsn string) (*sql.DB, error) {
	return OpenWithPool(ctx, dsn, Pool{MaxOpen: 8, MaxIdle: 4})
}

type Pool struct{ MaxOpen, MaxIdle int }

func PoolFromEnv(getenv func(string) string) (Pool, error) {
	p := Pool{MaxOpen: 8, MaxIdle: 4}
	for _, field := range []struct {
		name  string
		value *int
	}{{"OCP_DB_MAX_OPEN", &p.MaxOpen}, {"OCP_DB_MAX_IDLE", &p.MaxIdle}} {
		if raw := getenv(field.name); raw != "" {
			v, err := strconv.Atoi(raw)
			if err != nil {
				return Pool{}, errors.New("invalid database pool size")
			}
			*field.value = v
		}
	}
	if p.MaxOpen < 1 || p.MaxOpen > 64 || p.MaxIdle < 0 || p.MaxIdle > p.MaxOpen {
		return Pool{}, errors.New("database pool sizes exceed process budget")
	}
	return p, nil
}

func OpenWithPool(ctx context.Context, dsn string, pool Pool) (*sql.DB, error) {
	if pool.MaxOpen < 1 || pool.MaxOpen > 64 || pool.MaxIdle < 0 || pool.MaxIdle > pool.MaxOpen {
		return nil, errors.New("invalid database pool")
	}
	if dsn == "" {
		return nil, errors.New("database URL is required")
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, errors.New("invalid database connection configuration")
	}
	db.SetMaxOpenConns(pool.MaxOpen)
	db.SetMaxIdleConns(pool.MaxIdle)
	db.SetConnMaxLifetime(30 * time.Minute)
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, errors.New("database connection unavailable")
	}
	return db, nil
}

func Migrate(ctx context.Context, db *sql.DB) error {
	sub, err := fs.Sub(migrations, "migrations")
	if err != nil {
		return err
	}
	provider, err := goose.NewProvider(goose.DialectPostgres, db, sub)
	if err != nil {
		return err
	}
	_, err = provider.Up(ctx)
	return err
}

func TenantTransaction(ctx context.Context, db *sql.DB, merchant uuid.UUID) (*sql.Tx, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	if _, err = tx.ExecContext(ctx, `SELECT set_config('ocp.merchant_id',$1,true)`, merchant.String()); err != nil {
		tx.Rollback()
		return nil, err
	}
	return tx, nil
}

// CheckRuntimeRole rejects migration/administrative connections in the API.
func CheckRuntimeRole(ctx context.Context, db *sql.DB) error {
	var safe bool
	err := db.QueryRowContext(ctx, `SELECT NOT rolsuper AND NOT rolbypassrls
 AND NOT rolcreaterole AND NOT rolcreatedb AND rolname = 'ocp_app'
 AND NOT EXISTS (SELECT FROM pg_auth_members WHERE member=current_user::regrole)
 AND NOT EXISTS (SELECT FROM pg_roles elevated WHERE
   (elevated.rolsuper OR elevated.rolbypassrls OR elevated.rolcreaterole OR elevated.rolcreatedb)
   AND pg_has_role(current_user::regrole, elevated.oid, 'MEMBER'))
 AND NOT EXISTS (SELECT FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace
   WHERE n.nspname IN ('auth','commerce','payment','runtime','discovery','commerce_internal','shopping','lodging','payments') AND pg_has_role(current_user::regrole,c.relowner,'MEMBER'))
 AND NOT EXISTS (SELECT FROM pg_namespace n WHERE n.nspname IN ('auth','commerce','payment','runtime','discovery','commerce_internal','shopping','lodging','payments')
   AND pg_has_role(current_user::regrole,n.nspowner,'MEMBER'))
 AND NOT EXISTS (SELECT FROM pg_proc p JOIN pg_namespace n ON n.oid=p.pronamespace
   WHERE n.nspname IN ('auth','commerce','payment','runtime','discovery','commerce_internal','shopping','lodging','payments')
   AND p.prosecdef AND pg_has_role(current_user::regrole,p.proowner,'MEMBER'))
 AND NOT EXISTS (SELECT FROM pg_database d WHERE d.datname=current_database()
   AND pg_has_role(current_user::regrole,d.datdba,'MEMBER'))
 FROM pg_roles WHERE rolname = current_user`).Scan(&safe)
	if err != nil {
		return err
	}
	if !safe {
		return errors.New("API requires the restricted ocp_app role")
	}
	return nil
}
