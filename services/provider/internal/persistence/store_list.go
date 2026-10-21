package persistence

import (
	"context"
	"database/sql"
	"errors"

	"github.com/google/uuid"
)

type StorePage struct {
	Items     []Store    `json:"items"`
	NextAfter *uuid.UUID `json:"next_after,omitempty"`
}

// ListStores uses the same tenant transaction and restricted role as store reads.
// One extra row determines continuation without an unbounded count or offset scan.
func ListStores(ctx context.Context, db *sql.DB, merchant uuid.UUID, after *uuid.UUID, limit int) (StorePage, error) {
	page := StorePage{Items: make([]Store, 0)}
	if limit < 1 || limit > 100 {
		return page, errors.New("store page limit outside bounds")
	}
	tx, err := TenantTransaction(ctx, db, merchant)
	if err != nil {
		return page, err
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, `SELECT id,merchant_id,display_name,created_at
 FROM commerce.stores WHERE merchant_id=$1 AND ($2::uuid IS NULL OR id>$2)
 ORDER BY id LIMIT $3`, merchant, after, limit+1)
	if err != nil {
		return page, err
	}
	defer rows.Close()
	for rows.Next() {
		var store Store
		if err = rows.Scan(&store.ID, &store.MerchantID, &store.DisplayName, &store.CreatedAt); err != nil {
			return page, err
		}
		page.Items = append(page.Items, store)
	}
	if err = rows.Err(); err != nil {
		return page, err
	}
	if len(page.Items) > limit {
		page.Items = page.Items[:limit]
		id := page.Items[limit-1].ID
		page.NextAfter = &id
	}
	return page, tx.Commit()
}
