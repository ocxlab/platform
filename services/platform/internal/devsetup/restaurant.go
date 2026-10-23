// Package devsetup owns explicit development bootstraps, never HTTP operations.
package devsetup

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"ocp.local/platform/internal/infra/database"
	"regexp"

	"github.com/google/uuid"
	"ocp.local/platform/internal/core/persistence"
	"ocp.local/platform/internal/devfixtures"
	"ocp.local/platform/internal/httpapi"
	"ocp.local/platform/internal/offering"
	"ocp.local/platform/internal/offering/food"
)

var demoMerchant = uuid.MustParse("294e124d-5949-4f34-bae2-51caa7934df3")
var demoStore = uuid.MustParse("99598cf5-eded-4394-a053-751a1cb20db2")
var handlePattern = regexp.MustCompile(`^store-[1-9][0-9]{0,17}$`)

type StoreSummary struct {
	MerchantID   uuid.UUID `json:"merchant_id"`
	MerchantName string    `json:"merchant_name"`
	StoreID      uuid.UUID `json:"store_id"`
	Handle       string    `json:"store_handle"`
	Name         string    `json:"store_name"`
}

// Stores is available to the administrative CLI only, not an anonymous endpoint.
func Stores(ctx context.Context, db *sql.DB) ([]StoreSummary, error) {
	rows, err := db.QueryContext(ctx, `SELECT m.id,m.display_name,s.id,'store-'||s.display_number,s.display_name FROM auth.merchants m JOIN commerce.stores s ON s.merchant_id=m.id ORDER BY s.display_number`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []StoreSummary{}
	for rows.Next() {
		var item StoreSummary
		if err := rows.Scan(&item.MerchantID, &item.MerchantName, &item.StoreID, &item.Handle, &item.Name); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func ResolveMerchant(ctx context.Context, db *sql.DB, merchant, store string) (uuid.UUID, error) {
	if merchant != "" && store != "" {
		return uuid.Nil, errors.New("choose MERCHANT or STORE, not both")
	}
	if merchant != "" {
		id, err := uuid.Parse(merchant)
		if err != nil || id == uuid.Nil {
			return uuid.Nil, errors.New("MERCHANT must be a merchant UUID")
		}
		return id, nil
	}
	if !handlePattern.MatchString(store) {
		return uuid.Nil, errors.New("STORE must be a store handle such as store-1")
	}
	var id uuid.UUID
	err := db.QueryRowContext(ctx, `SELECT merchant_id FROM commerce.stores WHERE display_number=$1::bigint`, store[6:]).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return uuid.Nil, persistence.ErrNotFound
	}
	return id, err
}

// EnsureRestaurant creates its own fictional merchant and store once. Repeat runs
// preserve edits, publication state, purchases, other stores, and credentials.
func EnsureRestaurant(ctx context.Context, db *sql.DB) (persistence.Store, error) {
	menu, _, err := devfixtures.Restaurant()
	if err != nil {
		return persistence.Store{}, err
	}
	raw, err := json.Marshal(menu)
	if err != nil {
		return persistence.Store{}, err
	}
	spec, err := httpapi.GetSwagger()
	if err != nil {
		return persistence.Store{}, err
	}
	document, err := food.Validator(spec.Components.Schemas["FoodMenu"].Value)(raw)
	if err != nil {
		return persistence.Store{}, fmt.Errorf("demo menu: %w", err)
	}
	tx, err := database.TenantTransaction(ctx, db, demoMerchant)
	if err != nil {
		return persistence.Store{}, err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('ocp.demo.restaurant.v1',0))`); err != nil {
		return persistence.Store{}, err
	}
	var store persistence.Store
	store.ID, store.MerchantID = demoStore, demoMerchant
	err = tx.QueryRowContext(ctx, `SELECT display_name,created_at,'store-'||display_number FROM commerce.stores WHERE merchant_id=$1 AND id=$2`, demoMerchant, demoStore).Scan(&store.DisplayName, &store.CreatedAt, &store.Handle)
	if err == nil {
		return store, tx.Commit()
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return persistence.Store{}, err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO auth.merchants(id,display_name) VALUES($1,'Marktplatz Demo Gastronomie') ON CONFLICT(id) DO NOTHING`, demoMerchant); err != nil {
		return persistence.Store{}, err
	}
	store.DisplayName = "Marktplatz Küche (Demo)"
	err = tx.QueryRowContext(ctx, `INSERT INTO commerce.stores(merchant_id,id,display_name) VALUES($1,$2,$3) RETURNING created_at,'store-'||display_number`, demoMerchant, demoStore, store.DisplayName).Scan(&store.CreatedAt, &store.Handle)
	if err != nil {
		return persistence.Store{}, err
	}
	if _, err = persistence.AppendOffering(ctx, tx, demoMerchant, demoStore, 0, food.Model, document); err != nil {
		return persistence.Store{}, err
	}
	revision := int64(1)
	_, err = persistence.ChangePublication(ctx, tx, persistence.Principal{ID: demoMerchant, MerchantID: demoMerchant}, demoStore, 0, &revision, nil, "Explicit fictional development seed", uuid.New(), func(model offering.Model, raw json.RawMessage) (json.RawMessage, error) {
		if model != food.Model {
			return nil, offering.ErrInvalid
		}
		return food.Project(raw)
	})
	if err != nil {
		return persistence.Store{}, err
	}
	return store, tx.Commit()
}
