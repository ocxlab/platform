package persistence

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"ocp.local/platform/internal/core/persistence/sqlgen"
	"strconv"
	"strings"
)

var ErrAdminCursor = errors.New("invalid administration continuation")

type AdminPage struct {
	Items     []json.RawMessage `json:"items"`
	NextAfter *string           `json:"next_after,omitempty"`
}

// ListAdminRecords exposes summaries, never credentials, terms or payment details.
// Fixed function names are selected here; caller input never becomes SQL syntax.
func ListAdminRecords(ctx context.Context, db *sql.DB, p Principal, kind, query, state string, after *string, limit int) (AdminPage, error) {
	page := AdminPage{Items: make([]json.RawMessage, 0)}
	scope := "publication:read"
	if kind == "orders" {
		scope = "order:inspect"
	} else if kind != "stores" {
		return page, ErrAdminCursor
	}
	if !p.Provider || !p.Allows(scope) {
		return page, ErrUnauthenticated
	}
	query = strings.TrimSpace(query)
	key := fmt.Sprintf("%x", sha256.Sum256([]byte(kind+"\x00"+query+"\x00"+state)))
	var position sql.NullInt64
	if after != nil {
		parts := strings.Split(*after, ":")
		if len(parts) != 2 || parts[0] != key {
			return page, ErrAdminCursor
		}
		value, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil || value < 1 {
			return page, ErrAdminCursor
		}
		position = sql.NullInt64{Int64: value, Valid: true}
	}
	if limit < 1 || limit > 100 {
		return page, ErrAdminCursor
	}
	queries := sqlgen.New(db)
	args := sqlgen.ListAdminStoresParams{GrantID: p.ID, Query: query, State: state, AfterNumber: position, PageLimit: int32(limit + 1)}
	var rows []sqlgen.ListAdminStoresRow
	var err error
	if kind == "orders" {
		var orders []sqlgen.ListAdminOrdersRow
		orders, err = queries.ListAdminOrders(ctx, sqlgen.ListAdminOrdersParams(args))
		for _, order := range orders {
			rows = append(rows, sqlgen.ListAdminStoresRow(order))
		}
	} else {
		rows, err = queries.ListAdminStores(ctx, args)
	}
	if err != nil {
		return page, err
	}
	var last int64
	for _, row := range rows {
		if len(page.Items) == limit {
			cursor := key + ":" + strconv.FormatInt(last, 10)
			page.NextAfter = &cursor
			break
		}
		page.Items = append(page.Items, row.Body)
		last = row.SequenceNumber
	}
	return page, nil
}
