package persistence

import (
	"context"
	"database/sql"
	"errors"
	"ocp.local/platform/internal/infra/database"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"ocp.local/platform/internal/core/persistence/sqlgen"
)

type ManagedStore struct {
	Store
	DraftRevision int64     `json:"draft_revision"`
	MenuItemCount *int64    `json:"menu_item_count"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type StorePage struct {
	Items     []ManagedStore `json:"items"`
	NextAfter *uuid.UUID     `json:"next_after,omitempty"`
}

// ListStores uses the same tenant transaction and restricted role as store reads.
// One extra row determines continuation without an unbounded count or offset scan.
func ListStores(ctx context.Context, db *sql.DB, merchant uuid.UUID, after *uuid.UUID, limit int, handle *string) (StorePage, error) {
	page := StorePage{Items: make([]ManagedStore, 0)}
	var number *int64
	if handle != nil {
		value, err := strconv.ParseInt(strings.TrimPrefix(*handle, "store-"), 10, 64)
		if err != nil || value < 1 || *handle != "store-"+strconv.FormatInt(value, 10) {
			return page, errors.New("invalid store handle")
		}
		number = &value
	}
	if limit < 1 || limit > 100 {
		return page, errors.New("store page limit outside bounds")
	}
	tx, err := database.TenantTransaction(ctx, db, merchant)
	if err != nil {
		return page, err
	}
	defer tx.Rollback()
	params := sqlgen.ListManagedStoresParams{MerchantID: merchant, PageLimit: int32(limit + 1)}
	if after != nil {
		params.AfterID = uuid.NullUUID{UUID: *after, Valid: true}
	}
	if number != nil {
		params.DisplayNumber = sql.NullInt64{Int64: *number, Valid: true}
	}
	rows, err := sqlgen.New(tx).ListManagedStores(ctx, params)
	if err != nil {
		return page, err
	}
	for _, row := range rows {
		store := ManagedStore{Store: Store{ID: row.ID, MerchantID: row.MerchantID, DisplayName: row.DisplayName, CreatedAt: row.CreatedAt, Handle: row.Handle}, DraftRevision: row.OfferingRevision, UpdatedAt: row.UpdatedAt}
		// Counts cannot be negative. -1 represents a non-Food offering internally;
		// preserve the API's null count instead of treating it as an empty menu.
		if row.MenuItemCount >= 0 {
			count := row.MenuItemCount
			store.MenuItemCount = &count
		}
		page.Items = append(page.Items, store)
	}
	if len(page.Items) > limit {
		page.Items = page.Items[:limit]
		id := page.Items[limit-1].ID
		page.NextAfter = &id
	}
	return page, tx.Commit()
}
