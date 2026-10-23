// Package commerce owns authorized application operations shared by transports.
package commerce

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"regexp"

	"github.com/google/uuid"
	"ocp.local/platform/internal/core/offering"
	"ocp.local/platform/internal/core/persistence"
)

var ErrForbidden = errors.New("merchant action denied")
var ErrInvalid = errors.New("invalid application request")
var replayKey = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{15,127}$`)

// ValidRequestKey applies the shared mutation contract before database work.
func ValidRequestKey(key string) bool { return replayKey.MatchString(key) }

type Offerings struct {
	DB     *sql.DB
	Models *offering.Registry
}

func (s Offerings) Write(ctx context.Context, p persistence.Principal, merchant, store uuid.UUID, base int64, model offering.Model, document json.RawMessage, key string) (persistence.Result, error) {
	if !p.AuthorizesMerchant(merchant, "offering:write") {
		return persistence.Result{}, ErrForbidden
	}
	if base < 0 || store == uuid.Nil || !replayKey.MatchString(key) {
		return persistence.Result{}, ErrInvalid
	}
	canonicalDoc, err := s.Models.Validate(model, document)
	if err != nil {
		return persistence.Result{}, err
	}
	canonical, _ := json.Marshal(struct {
		Base     int64           `json:"base_revision"`
		Model    offering.Model  `json:"model"`
		Document json.RawMessage `json:"document"`
	}{base, model, canonicalDoc})
	return persistence.Mutate(ctx, s.DB, p, "writeOffering", store.String(), key, canonical, func(tx *sql.Tx) (any, error) {
		return persistence.AppendOffering(ctx, tx, merchant, store, base, model, canonicalDoc)
	})
}
