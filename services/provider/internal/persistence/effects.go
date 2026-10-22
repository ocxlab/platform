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
	"ocp.local/platform/internal/capability"
	"ocp.local/platform/internal/offering/food"
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
	var id uuid.UUID
	err := tx.QueryRowContext(ctx, `INSERT INTO runtime.effects(id,merchant_id,store_id,domain,kind,subject_id,effect_key,payload) VALUES($1,$2,$3,$4,$5,$6,$7,$8) ON CONFLICT(merchant_id,store_id,domain,kind,effect_key) DO NOTHING RETURNING id`, uuid.New(), e.MerchantID, e.StoreID, e.Domain, e.Kind, e.SubjectID, e.Key, string(e.Payload)).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		err = tx.QueryRowContext(ctx, `SELECT id FROM runtime.effects WHERE merchant_id=$1 AND store_id=$2 AND domain=$3 AND kind=$4 AND effect_key=$5 AND subject_id=$6 AND payload=$7::jsonb`, e.MerchantID, e.StoreID, e.Domain, e.Kind, e.Key, e.SubjectID, string(e.Payload)).Scan(&id)
	}
	return id, err
}

func validCatalogIntent(i CatalogIntent) bool {
	if i.Generation < 1 || i.Revision == nil || *i.Revision < 1 || len(i.DisplayName) == 0 || len(i.DisplayName) > 800 || (i.Model != food.Model && i.Model != food.ModelV2) {
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
	rows, err := db.QueryContext(ctx, `SELECT id,merchant_id,store_id,domain,kind,subject_id,effect_key,payload,lease_token,lease_generation,attempts,lease_until FROM runtime.claim_effects($1,$2,$3,$4)`, names, token, batch, int(lease/time.Second))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []Effect{}
	for rows.Next() {
		var e Effect
		if err := rows.Scan(&e.ID, &e.MerchantID, &e.StoreID, &e.Domain, &e.Kind, &e.SubjectID, &e.Key, &e.Payload, &e.Token, &e.Generation, &e.Attempts, &e.LeaseUntil); err != nil {
			return nil, err
		}
		result = append(result, e)
	}
	return result, rows.Err()
}
func FinishEffect(ctx context.Context, db *sql.DB, e Effect, success bool, retry time.Duration) (bool, error) {
	var done bool
	err := db.QueryRowContext(ctx, `SELECT runtime.finish_effect($1,$2,$3,$4,$5)`, e.ID, e.Token, e.Generation, success, int(retry/time.Second)).Scan(&done)
	return done, err
}
