-- name: LockUnitKey :exec
SELECT pg_advisory_xact_lock(hashtextextended(sqlc.arg(lock_key)::text,0));

-- name: ReadUnit :one
SELECT unit_id FROM lodging.units WHERE merchant_id=$1 AND store_id=$2 AND unit_id=$3;

-- name: StayOccupied :one
SELECT EXISTS(SELECT FROM lodging.allocations WHERE merchant_id=$1 AND store_id=$2 AND unit_id=$3 AND check_in<sqlc.arg(check_out)::text::date AND check_out>sqlc.arg(check_in)::text::date);

-- name: InsertBooking :exec
INSERT INTO lodging.bookings(merchant_id,store_id,id,session_id,terms) VALUES($1,$2,$3,$4,$5);

-- name: InsertAllocation :exec
INSERT INTO lodging.allocations(merchant_id,store_id,unit_id,booking_id,check_in,check_out) VALUES($1,$2,$3,$4,sqlc.arg(check_in)::text::date,sqlc.arg(check_out)::text::date);
