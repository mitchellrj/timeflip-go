#!/usr/bin/env bash
set -euo pipefail

root="$(git rev-parse --show-toplevel)"
cd "$root"

scripts/dev/pre-commit-go.sh

run_race="${RUN_GO_RACE:-auto}"
if [[ "$run_race" == "auto" && "$(go env GOOS)" != "darwin" ]]; then
  run_race=1
fi

if [[ "$run_race" == "1" ]]; then
  go test -count=1 -race ./...
else
  printf 'Skipping Go race tests on %s; set RUN_GO_RACE=1 to force them.\n' "$(go env GOOS)"
fi

mkdir -p build
go test -coverprofile=build/coverage.out ./...
go build -o build/timeflip-demo ./cmd/timeflip-demo
