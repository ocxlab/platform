package persistence

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"ocp.local/platform/internal/core/persistence/sqlgen"
)

var ErrPublicStoreCursor = errors.New("invalid public store cursor")

func GetPublicStoreByHandle(ctx context.Context, db *sql.DB, handle string) (PublicStore, error) {
	var store PublicStore
	number, err := strconv.ParseInt(strings.TrimPrefix(handle, "store-"), 10, 64)
	if err != nil || number < 1 || handle != "store-"+strconv.FormatInt(number, 10) {
		return store, ErrNotFound
	}
	row, err := sqlgen.New(db).GetPublicStoreByNumber(ctx, number)
	if errors.Is(err, sql.ErrNoRows) {
		return store, ErrNotFound
	}
	return PublicStore{MerchantID: row.MerchantID, ID: row.ID, DisplayName: row.DisplayName, Handle: row.Handle}, err
}

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
	var merchant, store uuid.NullUUID
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
		merchant, store = uuid.NullUUID{UUID: m, Valid: true}, uuid.NullUUID{UUID: s, Valid: true}
	}
	if limit < 1 || limit > 100 {
		return page, ErrPublicStoreCursor
	}
	rows, err := sqlgen.New(db).ListPublicStores(ctx, sqlgen.ListPublicStoresParams{Query: query, AfterMerchant: merchant, AfterStore: store, PageLimit: int32(limit + 1)})
	if err != nil {
		return page, err
	}
	for _, row := range rows {
		page.Items = append(page.Items, PublicStore{MerchantID: row.MerchantID, ID: row.ID, DisplayName: row.DisplayName, Handle: row.Handle})
	}
	if len(page.Items) > limit {
		page.Items = page.Items[:limit]
		last := page.Items[limit-1]
		cursor := queryKey + ":" + last.MerchantID.String() + ":" + last.ID.String()
		page.NextAfter = &cursor
	}
	return page, nil
}
