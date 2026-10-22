package persistence

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
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
	tx, err := TenantTransaction(ctx, db, merchant)
	if err != nil {
		return page, err
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, `SELECT s.id,s.merchant_id,s.display_name,s.created_at,
 s.offering_revision,
 CASE WHEN s.offering_revision=0 THEN 0
 WHEN r.model_id='ocp.food.menu' AND r.model_version IN ('1.0.0','2.0.0')
 AND jsonb_typeof(r.document->'items')='array' THEN jsonb_array_length(r.document->'items')
 ELSE NULL END,
 COALESCE(r.created_at,s.created_at),'store-'||s.display_number
 FROM commerce.stores s LEFT JOIN commerce.offering_revisions r
 ON r.merchant_id=s.merchant_id AND r.store_id=s.id AND r.revision=s.offering_revision
 WHERE s.merchant_id=$1 AND ($2::uuid IS NULL OR s.id>$2)
 AND ($4::bigint IS NULL OR s.display_number=$4)
 ORDER BY s.id LIMIT $3`, merchant, after, limit+1, number)
	if err != nil {
		return page, err
	}
	defer rows.Close()
	for rows.Next() {
		var store ManagedStore
		if err = rows.Scan(&store.ID, &store.MerchantID, &store.DisplayName, &store.CreatedAt, &store.DraftRevision, &store.MenuItemCount, &store.UpdatedAt, &store.Handle); err != nil {
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
