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
