#!/bin/sh
set -eu
trial_dir=$(mktemp -d)
trap 'rm -r "$trial_dir"' EXIT
mkdir -p "$trial_dir/internal/infra/database"
cp -R internal/infra/database/migrations "$trial_dir/internal/infra/database/"
query_owners='internal/core internal/food internal/shopping internal/lodging cmd/api/transport/ucpinternal internal/payment internal/discovery'
for owner in $query_owners; do
  mkdir -p "$trial_dir/$owner/persistence"
  cp -R "$owner/persistence/queries" "$trial_dir/$owner/persistence/"
done
cp sqlc.yaml "$trial_dir/sqlc.yaml"
sqlc generate -f "$trial_dir/sqlc.yaml"
for owner in $query_owners; do
  diff -ru "$owner/persistence/sqlgen" "$trial_dir/$owner/persistence/sqlgen"
done
