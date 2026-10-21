package persistence

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"strings"

	"github.com/google/uuid"
)

var ErrPublicStoreCursor = errors.New("invalid public store cursor")

type PublicStore struct {
	Handle      string    `json:"handle,omitempty"`
	MerchantID  uuid.UUID `json:"merchant_id"`
	ID          uuid.UUID `json:"id"`
	DisplayName string    `json:"display_name"`
}
type PublicStorePage struct {
	Items     []PublicStore `json:"items"`
	NextAfter *string       `json:"next_after,omitempty"`
}

func ListPublicStores(ctx context.Context, db *sql.DB, query string, after *string, limit int) (PublicStorePage, error) {
	page := PublicStorePage{Items: make([]PublicStore, 0)}
	query = strings.TrimSpace(query)
	digest := sha256.Sum256([]byte(query))
	queryKey := hex.EncodeToString(digest[:])
	var merchant, store *uuid.UUID
	if after != nil {
		parts := strings.Split(*after, ":")
		if len(parts) != 3 || parts[0] != queryKey {
			return page, ErrPublicStoreCursor
		}
		m, e1 := uuid.Parse(parts[1])
		s, e2 := uuid.Parse(parts[2])
		if e1 != nil || e2 != nil {
			return page, ErrPublicStoreCursor
		}
		merchant, store = &m, &s
	}
	if limit < 1 || limit > 100 {
		return page, ErrPublicStoreCursor
	}
	rows, err := db.QueryContext(ctx, `SELECT merchant_id,id,display_name,handle FROM commerce.list_public_stores($1,$2,$3,$4)`, strings.TrimSpace(query), merchant, store, limit+1)
	if err != nil {
		return page, err
	}
	defer rows.Close()
	for rows.Next() {
		var entry PublicStore
		if err = rows.Scan(&entry.MerchantID, &entry.ID, &entry.DisplayName, &entry.Handle); err != nil {
			return page, err
		}
		page.Items = append(page.Items, entry)
	}
	if err = rows.Err(); err != nil {
		return page, err
	}
	if len(page.Items) > limit {
		page.Items = page.Items[:limit]
		last := page.Items[limit-1]
		cursor := queryKey + ":" + last.MerchantID.String() + ":" + last.ID.String()
		page.NextAfter = &cursor
	}
	return page, nil
}
