package persistence

import (
	"context"
	"database/sql"
	"errors"
	corestore "ocp.local/platform/internal/core/persistence"
	"ocp.local/platform/internal/database"

	"github.com/google/uuid"
	"ocp.local/platform/internal/food/persistence/sqlgen"
)

var ErrOrderingPaused = errors.New("new food ordering paused")

type OrderingRevisionConflict struct{ Current int64 }

func (e *OrderingRevisionConflict) Error() string { return "ordering settings revision conflict" }

type FoodOrdering struct {
	Revision        int64  `json:"revision"`
	AcceptingOrders bool   `json:"accepting_orders"`
	AcceptanceMode  string `json:"acceptance_mode"`
	actor           uuid.NullUUID
}

// Callers taking a lock use the same store row as publication, quotes, and placement.
func foodOrdering(ctx context.Context, tx *sql.Tx, merchant, store uuid.UUID, lock bool) (FoodOrdering, error) {
	return readFoodOrdering(ctx, tx, merchant, store, lock, false)
}

func readFoodOrdering(ctx context.Context, tx *sql.Tx, merchant, store uuid.UUID, lock, public bool) (FoodOrdering, error) {
	queries := sqlgen.New(tx)
	args := sqlgen.GetFoodOrderingParams{MerchantID: merchant, ID: store, PublicOnly: public}
	var row sqlgen.GetFoodOrderingRow
	var err error
	if lock {
		var locked sqlgen.LockFoodOrderingRow
		locked, err = queries.LockFoodOrdering(ctx, sqlgen.LockFoodOrderingParams(args))
		row = sqlgen.GetFoodOrderingRow(locked)
	} else {
		row, err = queries.GetFoodOrdering(ctx, args)
	}
	if errors.Is(err, sql.ErrNoRows) {
		err = corestore.ErrNotFound
	}
	return FoodOrdering{Revision: row.OrderingRevision, AcceptingOrders: row.AcceptingOrders, AcceptanceMode: row.AcceptanceMode, actor: row.OrderingActorID}, err
}

func GetFoodOrdering(ctx context.Context, db *sql.DB, merchant, store uuid.UUID, public bool) (FoodOrdering, error) {
	tx, err := database.TenantTransaction(ctx, db, merchant)
	if err != nil {
		return FoodOrdering{}, err
	}
	defer tx.Rollback()
	// One row snapshot checks visibility and reads settings without loading the
	// menu or taking an exclusive write lock for an advisory public read.
	state, err := readFoodOrdering(ctx, tx, merchant, store, false, public)
	if err != nil {
		return state, err
	}
	return state, tx.Commit()
}

func ChangeFoodOrdering(ctx context.Context, tx *sql.Tx, actor corestore.Principal, store uuid.UUID, expected int64, accepting bool, mode string, requestID uuid.UUID) (FoodOrdering, error) {
	state, err := foodOrdering(ctx, tx, actor.MerchantID, store, true)
	if err != nil {
		return state, err
	}
	if expected != state.Revision {
		return state, &OrderingRevisionConflict{state.Revision}
	}
	if mode != "manual" && mode != "automatic" {
		return state, errors.New("invalid acceptance mode")
	}
	state.Revision++
	state.AcceptingOrders = accepting
	state.AcceptanceMode = mode
	queries := sqlgen.New(tx)
	err = queries.UpdateFoodOrdering(ctx, sqlgen.UpdateFoodOrderingParams{MerchantID: actor.MerchantID, ID: store, OrderingRevision: state.Revision, AcceptingOrders: accepting, AcceptanceMode: mode, OrderingActorID: uuid.NullUUID{UUID: actor.ID, Valid: true}})
	if err != nil {
		return state, err
	}
	err = queries.InsertFoodOrderingAudit(ctx, sqlgen.InsertFoodOrderingAuditParams{MerchantID: actor.MerchantID, StoreID: store, Revision: state.Revision, AcceptingOrders: accepting, AcceptanceMode: mode, ActorID: actor.ID, RequestID: requestID})
	return state, err
}
