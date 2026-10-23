#!/bin/sh
set -eu
# Only an isolated module copy is mutated; no development process/source changes.
mutation_dir=$(mktemp -d /tmp/ocp-work-mutation.XXXXXX)
trap 'rm -rf "$mutation_dir"' EXIT
cp -R /workspace/services/platform/. "$mutation_dir/"
mkdir -p /workspace/reports/mutation/go
cd "$mutation_dir"
# SQL adapters and the live PostgreSQL listener require the integration suite.
# Keep this bounded run on the scheduler and attempt/cleanup logic with unit fakes.
go run github.com/go-gremlins/gremlins/cmd/gremlins@v0.6.0 unleash ./internal/core/work \
  --workers 2 --exclude-files '(store|stats|wake)\.go$' \
  --output /workspace/reports/mutation/go/work.json
