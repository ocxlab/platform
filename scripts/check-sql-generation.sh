#!/bin/sh
set -eu
trial_dir=$(mktemp -d)
trap 'rm -r "$trial_dir"' EXIT
# Keep inputs relative to the temporary config, as required by sqlc.
mkdir -p "$trial_dir/internal/persistence"
cp -R internal/persistence/migrations internal/persistence/queries "$trial_dir/internal/persistence/"
sed 's@out: internal/persistence/sqlgen@out: sqlgen@' sqlc.yaml > "$trial_dir/sqlc.yaml"
sqlc generate -f "$trial_dir/sqlc.yaml"
diff -ru internal/persistence/sqlgen "$trial_dir/sqlgen"
