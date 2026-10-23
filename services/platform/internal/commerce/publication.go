package commerce

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/google/uuid"
	"ocp.local/platform/internal/core/persistence"
	"ocp.local/platform/internal/offering"
)

type Publications struct {
	DB     *sql.DB
	Models *offering.Registry
}

func (s Publications) Change(ctx context.Context, p persistence.Principal, merchant, store uuid.UUID, expected int64, revision *int64, key string, requestID uuid.UUID) (persistence.Result, error) {
	if !p.AuthorizesMerchant(merchant, "offering:write") {
		return persistence.Result{}, ErrForbidden
	}
	if store == uuid.Nil || expected < 0 || (revision != nil && *revision < 1) || !replayKey.MatchString(key) {
		return persistence.Result{}, ErrInvalid
	}
	canonical, _ := json.Marshal(struct {
		Expected int64  `json:"expected_generation"`
		Revision *int64 `json:"revision"`
	}{expected, revision})
	return persistence.Mutate(ctx, s.DB, p, "changePublication", store.String(), key, canonical, func(tx *sql.Tx) (any, error) {
		return persistence.ChangePublication(ctx, tx, p, store, expected, revision, nil, "", requestID, s.Models.Project)
	})
}
