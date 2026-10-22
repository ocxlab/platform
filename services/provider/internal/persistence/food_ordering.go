package persistence

import (
	"context"
	"database/sql"
	"errors"

	"github.com/google/uuid"
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
	var state FoodOrdering
	query := `SELECT ordering_revision,accepting_orders,acceptance_mode,ordering_actor_id FROM commerce.stores WHERE merchant_id=$1 AND id=$2`
	if public {
		query += ` AND published_revision IS NOT NULL AND NOT publication_suspended`
	}
	if lock {
		query += ` FOR UPDATE`
	}
	err := tx.QueryRowContext(ctx, query, merchant, store).Scan(&state.Revision, &state.AcceptingOrders, &state.AcceptanceMode, &state.actor)
	if errors.Is(err, sql.ErrNoRows) {
		err = ErrNotFound
	}
	return state, err
}

func GetFoodOrdering(ctx context.Context, db *sql.DB, merchant, store uuid.UUID, public bool) (FoodOrdering, error) {
	tx, err := TenantTransaction(ctx, db, merchant)
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

func ChangeFoodOrdering(ctx context.Context, tx *sql.Tx, actor Principal, store uuid.UUID, expected int64, accepting bool, mode string, requestID uuid.UUID) (FoodOrdering, error) {
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
	_, err = tx.ExecContext(ctx, `UPDATE commerce.stores SET ordering_revision=$3,accepting_orders=$4,acceptance_mode=$5,ordering_actor_id=$6 WHERE merchant_id=$1 AND id=$2`, actor.MerchantID, store, state.Revision, accepting, mode, actor.ID)
	if err != nil {
		return state, err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO commerce.food_ordering_audit(merchant_id,store_id,revision,accepting_orders,acceptance_mode,actor_id,request_id) VALUES($1,$2,$3,$4,$5,$6,$7)`, actor.MerchantID, store, state.Revision, accepting, mode, actor.ID, requestID)
	return state, err
}
