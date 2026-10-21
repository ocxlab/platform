#!/bin/sh
set -eu
# Gremlins mutates a throwaway copy, never the running development source.
# Copy only the owned module; no .env, research snapshots, or external keys.
mutation_dir=$(mktemp -d /tmp/ocp-mutation.XXXXXX)
trap 'rm -rf "$mutation_dir"' EXIT
cp -R /workspace/services/provider/. "$mutation_dir/"
mkdir -p /workspace/reports/mutation/go
cd "$mutation_dir"
go run github.com/go-gremlins/gremlins/cmd/gremlins@v0.6.0 unleash ./internal/payment --workers 2 --output /workspace/reports/mutation/go/payment.json
go run github.com/go-gremlins/gremlins/cmd/gremlins@v0.6.0 unleash ./internal/offering/food --workers 2 --output /workspace/reports/mutation/go/food.json
