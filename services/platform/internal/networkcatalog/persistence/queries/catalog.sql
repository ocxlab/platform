-- name: AcceptCatalog :exec
SELECT discovery.accept_catalog($1,$2,$3,$4,$5,$6,$7,$8);

-- name: SearchCatalogs :many
SELECT body,observed_at FROM discovery.catalogs WHERE (body->>'visible')::boolean AND observed_at>clock_timestamp()-interval '24 hours' AND position(lower($1) in lower(body->>'display_name'))>0 ORDER BY store_id LIMIT 100;
