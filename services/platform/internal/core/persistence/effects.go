package persistence

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"time"

	"github.com/google/uuid"
	"ocp.local/platform/internal/core/capability"
	"ocp.local/platform/internal/core/offering"
	"ocp.local/platform/internal/core/persistence/sqlgen"
)

type Effect struct {
	ID         uuid.UUID
	MerchantID uuid.UUID
	StoreID    uuid.UUID
	Domain     capability.Domain
	Kind       string
	SubjectID  uuid.UUID
	Key        string
	Payload    json.RawMessage
	Token      uuid.UUID
	Generation int64
	Attempts   int
	LeaseUntil time.Time
}

// SupportedEffect is extended only alongside an owned handler and its contract.
func SupportedEffect(domain capability.Domain, kind string) bool {
	return domain == capability.Food && kind == "catalog.publish"
}

// EnqueueEffect belongs inside the domain decision transaction. A stable key
// deduplicates retries; a changed payload with the same identity is a conflict.
func EnqueueEffect(ctx context.Context, tx *sql.Tx, e Effect) (uuid.UUID, error) {
	var object map[string]any
	if !SupportedEffect(e.Domain, e.Kind) || len(e.Key) == 0 || len(e.Key) > 200 || e.MerchantID == uuid.Nil || e.StoreID == uuid.Nil || e.SubjectID == uuid.Nil || json.Unmarshal(e.Payload, &object) != nil || object == nil {
		return uuid.Nil, errors.New("invalid effect")
	}
	var intent CatalogIntent
	decoder := json.NewDecoder(bytes.NewReader(e.Payload))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&intent) != nil || decoder.Decode(new(any)) != io.EOF || !validCatalogIntent(intent) {
		return uuid.Nil, errors.New("invalid catalog intent")
	}
	e.Payload, _ = json.Marshal(intent)
	queries := sqlgen.New(tx)
	id, err := queries.InsertEffect(ctx, sqlgen.InsertEffectParams{ID: uuid.New(), MerchantID: e.MerchantID, StoreID: e.StoreID, Domain: string(e.Domain), Kind: e.Kind, SubjectID: e.SubjectID, EffectKey: e.Key, Payload: e.Payload})
	if errors.Is(err, sql.ErrNoRows) {
		id, err = queries.GetMatchingEffect(ctx, sqlgen.GetMatchingEffectParams{MerchantID: e.MerchantID, StoreID: e.StoreID, Domain: string(e.Domain), Kind: e.Kind, EffectKey: e.Key, SubjectID: e.SubjectID, Payload: e.Payload})
	}
	return id, err
}

func validCatalogIntent(i CatalogIntent) bool {
	if i.Generation < 1 || i.Revision == nil || *i.Revision < 1 || len(i.DisplayName) == 0 || len(i.DisplayName) > 800 || i.Model != offering.FoodMenu {
		return false
	}
	switch i.Action {
	case "publish":
	case "restore":
		if !i.Visible {
			return false
		}
	case "withdraw", "suspend":
		if i.Visible {
			return false
		}
	default:
		return false
	}
	var doc map[string]any
	if i.Visible {
		return json.Unmarshal(i.Document, &doc) == nil && doc != nil
	}
	return bytes.Equal(bytes.TrimSpace(i.Document), []byte("null"))
}

func ClaimEffects(ctx context.Context, db *sql.DB, domains []capability.Domain, batch int, lease time.Duration) ([]Effect, error) {
	names := []string{}
	for _, d := range domains {
		if !d.Valid() {
			return nil, errors.New("invalid domain")
		}
		names = append(names, string(d))
	}
	token := uuid.New()
	domainsJSON, err := json.Marshal(names)
	if err != nil {
		return nil, err
	}
	rows, err := sqlgen.New(db).ClaimEffects(ctx, sqlgen.ClaimEffectsParams{DomainsJson: domainsJSON, Token: token, BatchSize: int64(batch), LeaseSeconds: int64(lease / time.Second)})
	if err != nil {
		return nil, err
	}
	result := []Effect{}
	for _, row := range rows {
		if !row.LeaseToken.Valid || !row.LeaseUntil.Valid {
			return nil, errors.New("claimed effect has no lease")
		}
		e := Effect{ID: row.ID, MerchantID: row.MerchantID, StoreID: row.StoreID, Domain: capability.Domain(row.Domain), Kind: row.Kind, SubjectID: row.SubjectID, Key: row.EffectKey, Payload: row.Payload, Token: row.LeaseToken.UUID, Generation: row.LeaseGeneration, Attempts: int(row.Attempts), LeaseUntil: row.LeaseUntil.Time}
		result = append(result, e)
	}
	return result, nil
}
func FinishEffect(ctx context.Context, db *sql.DB, e Effect, success bool, retry time.Duration) (bool, error) {
	return sqlgen.New(db).FinishEffect(ctx, sqlgen.FinishEffectParams{Effect: e.ID, Token: e.Token, Generation: e.Generation, Success: success, RetrySeconds: int64(retry / time.Second)})
}
