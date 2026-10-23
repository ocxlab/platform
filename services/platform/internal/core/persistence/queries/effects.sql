-- name: InsertEffect :one
INSERT INTO runtime.effects(id,merchant_id,store_id,domain,kind,subject_id,effect_key,payload)
VALUES($1,$2,$3,$4,$5,$6,$7,$8)
ON CONFLICT(merchant_id,store_id,domain,kind,effect_key) DO NOTHING RETURNING id;

-- name: GetMatchingEffect :one
SELECT id FROM runtime.effects
WHERE merchant_id=sqlc.arg(merchant_id) AND store_id=sqlc.arg(store_id)
AND domain=sqlc.arg(domain) AND kind=sqlc.arg(kind)
AND effect_key=sqlc.arg(effect_key) AND subject_id=sqlc.arg(subject_id)
AND payload=sqlc.arg(payload)::jsonb;

-- name: ClaimEffects :many
SELECT id,merchant_id,store_id,domain,kind,subject_id,effect_key,payload,lease_token,lease_generation,attempts,lease_until
FROM runtime.claim_effects(ARRAY(SELECT jsonb_array_elements_text(sqlc.arg(domains_json)::jsonb)),sqlc.arg(token)::uuid,sqlc.arg(batch_size)::bigint::integer,sqlc.arg(lease_seconds)::bigint::integer);

-- name: FinishEffect :one
SELECT runtime.finish_effect($1,$2,$3,$4,sqlc.arg(retry_seconds)::bigint::integer)::boolean AS done;

-- name: NextEffectDelay :one
SELECT runtime.next_effect_delay(ARRAY(SELECT jsonb_array_elements_text(sqlc.arg(domains_json)::jsonb)))::double precision AS seconds;
