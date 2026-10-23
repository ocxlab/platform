#!/bin/sh
set -eu
# Mutate an isolated module copy, never the development application sources.
mutation_dir=$(mktemp -d /tmp/ocp-mcp-mutation.XXXXXX)
trap 'rm -rf "$mutation_dir"' EXIT
cp -R /workspace/services/platform/. "$mutation_dir/"
mkdir -p /workspace/reports/mutation/go
cd "$mutation_dir"
# Focus on price edits, meal-offer projections/edits, and retained diffs.
# HTTP/SQL callbacks are integration-tested separately, not covered by these mutants.
go run github.com/go-gremlins/gremlins/cmd/gremlins@v0.6.0 unleash ./internal/food/adapters/mcp \
  --workers 2 --exclude-files '(merchant|menu_details)\.go$' \
  --output /workspace/reports/mutation/go/mcp.json
