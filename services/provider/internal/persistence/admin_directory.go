package persistence

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
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
	statement := `SELECT body,sequence_number FROM commerce.admin_stores($1,$2,$3,$4,$5)`
	if kind == "orders" {
		scope = "order:inspect"
		statement = `SELECT body,sequence_number FROM commerce.admin_orders($1,$2,$3,$4,$5)`
	} else if kind != "stores" {
		return page, ErrAdminCursor
	}
	if !p.Provider || !p.Allows(scope) {
		return page, ErrUnauthenticated
	}
	query = strings.TrimSpace(query)
	key := fmt.Sprintf("%x", sha256.Sum256([]byte(kind+"\x00"+query+"\x00"+state)))
	var position *int64
	if after != nil {
		parts := strings.Split(*after, ":")
		if len(parts) != 2 || parts[0] != key {
			return page, ErrAdminCursor
		}
		value, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil || value < 1 {
			return page, ErrAdminCursor
		}
		position = &value
	}
	if limit < 1 || limit > 100 {
		return page, ErrAdminCursor
	}
	rows, err := db.QueryContext(ctx, statement, p.ID, query, state, position, limit+1)
	if err != nil {
		return page, err
	}
	defer rows.Close()
	var last int64
	for rows.Next() {
		var body json.RawMessage
		var number int64
		if err = rows.Scan(&body, &number); err != nil {
			return page, err
		}
		if len(page.Items) == limit {
			cursor := key + ":" + strconv.FormatInt(last, 10)
			page.NextAfter = &cursor
			break
		}
		page.Items = append(page.Items, body)
		last = number
	}
	return page, rows.Err()
}
