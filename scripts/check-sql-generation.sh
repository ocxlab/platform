#!/bin/sh
set -eu
trial_dir=$(mktemp -d)
trap 'rm -r "$trial_dir"' EXIT
mkdir -p "$trial_dir/internal/database"
cp -R internal/database/migrations "$trial_dir/internal/database/"
for owner in core food shopping lodging ucpinternal payment networkcatalog; do
  mkdir -p "$trial_dir/internal/$owner/persistence"
  cp -R "internal/$owner/persistence/queries" "$trial_dir/internal/$owner/persistence/"
done
cp sqlc.yaml "$trial_dir/sqlc.yaml"
sqlc generate -f "$trial_dir/sqlc.yaml"
for owner in core food shopping lodging ucpinternal payment networkcatalog; do
  diff -ru "internal/$owner/persistence/sqlgen" "$trial_dir/internal/$owner/persistence/sqlgen"
done
