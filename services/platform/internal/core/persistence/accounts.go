package persistence

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"ocp.local/platform/internal/core/persistence/sqlgen"
	"ocp.local/platform/internal/infra/database"
	"slices"
	"strings"
	"time"
)

// AccountPrincipal deliberately cannot be passed as commerce Principal.
type AccountPrincipal struct {
	ID, AccountID  uuid.UUID
	Kind, Resource string
	Scopes         []string
}

func (p AccountPrincipal) Allows(scope string) bool { return slices.Contains(p.Scopes, scope) }
func AccountResource(kind, transport string) string {
	if (kind != "merchant" && kind != "customer") || (transport != "rest" && transport != "mcp") {
		return ""
	}
	return kind + "-account-" + transport
}

type AccountProfile struct {
	ID          uuid.UUID `json:"id"`
	Kind        string    `json:"kind"`
	DisplayName string    `json:"display_name"`
	Locale      string    `json:"locale"`
	Revision    int64     `json:"revision"`
	UpdatedAt   time.Time `json:"updated_at"`
}
type AccountRevisionConflict struct{ Current int64 }

func (e *AccountRevisionConflict) Error() string { return "account revision conflict" }

func AuthenticateAccount(ctx context.Context, db *sql.DB, token, resource string) (AccountPrincipal, error) {
	if len(token) != 47 || !strings.HasPrefix(token, "oca_") || !slices.Contains([]string{"merchant-account-rest", "customer-account-rest", "merchant-account-mcp", "customer-account-mcp"}, resource) {
		return AccountPrincipal{}, ErrUnauthenticated
	}
	digest := sha256.Sum256([]byte(token))
	row, err := sqlgen.New(db).LookupAccountGrant(ctx, sqlgen.LookupAccountGrantParams{Candidate: digest[:], RequestedResource: resource})
	if errors.Is(err, sql.ErrNoRows) {
		return AccountPrincipal{}, ErrUnauthenticated
	}
	if err != nil {
		return AccountPrincipal{}, err
	}
	p := AccountPrincipal{ID: row.ID, AccountID: row.AccountID, Kind: row.Kind, Resource: resource}
	err = json.Unmarshal([]byte(row.ScopesJson), &p.Scopes)
	return p, err
}

// Administrative local/test bootstrap. Public signup, identity verification, and
// OAuth token issuance are intentionally not implemented by this helper.
func ProvisionAccount(ctx context.Context, db *sql.DB, kind, name string, merchant uuid.UUID) (uuid.UUID, error) {
	if (kind != "merchant" && kind != "customer") || (kind == "merchant") != (merchant != uuid.Nil) || strings.TrimSpace(name) == "" {
		return uuid.Nil, ErrNotFound
	}
	id := uuid.New()
	err := sqlgen.New(db).InsertAccount(ctx, sqlgen.InsertAccountParams{ID: id, Kind: kind, MerchantID: uuid.NullUUID{UUID: merchant, Valid: merchant != uuid.Nil}, DisplayName: name, Locale: "en"})
	return id, err
}
func ProvisionAccountGrant(ctx context.Context, db *sql.DB, account uuid.UUID, kind, transport string, scopes []string, expiry time.Time) (string, error) {
	resource := AccountResource(kind, transport)
	if resource == "" || !expiry.After(time.Now()) || expiry.After(time.Now().Add(24*time.Hour)) || len(scopes) == 0 {
		return "", ErrUnauthenticated
	}
	for _, scope := range scopes {
		if scope != "account:read" && scope != "account:write" {
			return "", ErrUnauthenticated
		}
	}
	var entropy [32]byte
	profile, e := sqlgen.New(db).ReadAccountProfile(ctx, account)
	if e != nil {
		return "", e
	}
	if profile.Kind != kind {
		return "", ErrUnauthenticated
	}
	if _, err := rand.Read(entropy[:]); err != nil {
		return "", err
	}
	token := "oca_" + base64.RawURLEncoding.EncodeToString(entropy[:])
	digest := sha256.Sum256([]byte(token))
	encoded, _ := json.Marshal(scopes)
	err := sqlgen.New(db).InsertAccountGrant(ctx, sqlgen.InsertAccountGrantParams{ID: uuid.New(), AccountID: account, TokenHash: digest[:], Resource: resource, ScopesJson: encoded, ExpiresAt: expiry})
	return token, err
}
func accountQueries(ctx context.Context, db *sql.DB, p AccountPrincipal, scope string) (*sql.Tx, *sqlgen.Queries, error) {
	if p.ID == uuid.Nil || p.AccountID == uuid.Nil || !p.Allows(scope) || !strings.HasPrefix(p.Resource, p.Kind+"-account-") {
		return nil, nil, ErrUnauthenticated
	}
	tx, err := database.AccountTransaction(ctx, db, p.AccountID)
	if err != nil {
		return nil, nil, err
	}
	q := sqlgen.New(tx)
	if scope == "account:write" {
		if _, err = q.LockAccountGrant(ctx, sqlgen.LockAccountGrantParams{AccountID: p.AccountID, ID: p.ID, Resource: p.Resource, RequiredScope: "account:read"}); err != nil {
			tx.Rollback()
			if errors.Is(err, sql.ErrNoRows) {
				err = ErrUnauthenticated
			}
			return nil, nil, err
		}
	}
	if _, err = q.LockAccountGrant(ctx, sqlgen.LockAccountGrantParams{AccountID: p.AccountID, ID: p.ID, Resource: p.Resource, RequiredScope: scope}); err != nil {
		tx.Rollback()
		if errors.Is(err, sql.ErrNoRows) {
			err = ErrUnauthenticated
		}
		return nil, nil, err
	}
	return tx, q, nil
}
func ReadAccount(ctx context.Context, db *sql.DB, p AccountPrincipal) (AccountProfile, error) {
	tx, q, err := accountQueries(ctx, db, p, "account:read")
	if err != nil {
		return AccountProfile{}, err
	}
	defer tx.Rollback()
	row, err := q.ReadAccountProfile(ctx, p.AccountID)
	if err != nil {
		return AccountProfile{}, err
	}
	return AccountProfile(row), tx.Commit()
}
func ChangeAccount(ctx context.Context, db *sql.DB, p AccountPrincipal, revision int64, name, locale, key string, request uuid.UUID) (Result, error) {
	tx, q, err := accountQueries(ctx, db, p, "account:write")
	if err != nil {
		return Result{}, err
	}
	defer tx.Rollback()
	// Account row serializes edits and retry-key checks, so a concurrent retry
	// cannot perform a second mutation. Replays are checked before revision guards.
	current, err := q.LockAccountProfile(ctx, p.AccountID)
	if err != nil {
		return Result{}, err
	}
	canonical, _ := json.Marshal(struct {
		Revision     int64
		Name, Locale string
	}{revision, name, locale})
	digest := sha256.Sum256(canonical)
	previous, err := q.ReadAccountReplay(ctx, sqlgen.ReadAccountReplayParams{AccountID: p.AccountID, GrantID: p.ID, ReplayKey: key})
	if err == nil {
		if !bytes.Equal(digest[:], previous.RequestHash) {
			return Result{}, ErrReplayConflict
		}
		return Result{Status: 200, Body: previous.ResponseBody, Replayed: true}, tx.Commit()
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return Result{}, err
	}
	if current.Revision != revision {
		return Result{}, &AccountRevisionConflict{Current: current.Revision}
	}
	row, err := q.UpdateAccountProfile(ctx, sqlgen.UpdateAccountProfileParams{ID: p.AccountID, DisplayName: name, Locale: locale})
	if err != nil {
		return Result{}, err
	}
	encoded, err := json.Marshal(AccountProfile(row))
	if err != nil {
		return Result{}, err
	}
	if err = q.InsertAccountAudit(ctx, sqlgen.InsertAccountAuditParams{AccountID: p.AccountID, Revision: row.Revision, GrantID: p.ID, RequestID: request}); err != nil {
		return Result{}, err
	}
	if err = q.InsertAccountReplay(ctx, sqlgen.InsertAccountReplayParams{AccountID: p.AccountID, GrantID: p.ID, ReplayKey: key, RequestHash: digest[:], ResponseBody: encoded}); err != nil {
		return Result{}, err
	}
	return Result{Status: 200, Body: encoded}, tx.Commit()
}
