-- name: InsertPayLaterObligation :exec
INSERT INTO payments.obligations(merchant_id,store_id,domain,subject_id,amount_minor,currency,condition,state) VALUES($1,$2,$3,$4,$5,$6,'pay_later','outstanding');
