#!/usr/bin/env bash
set -euo pipefail

root="$(git rev-parse --show-toplevel)"
cd "$root"

check_gofmt() {
  local files
  files="$(find . -path ./build -prune -o -name '*.go' -print0 | xargs -0 gofmt -l)"
  if [[ -n "$files" ]]; then
    printf 'Go files need gofmt:\n%s\n' "$files" >&2
    return 1
  fi
}

check_go_mod_tidy() {
  local tmpdir
  tmpdir="$(mktemp -d)"
  cp go.mod "$tmpdir/go.mod"
  cp go.sum "$tmpdir/go.sum"

  restore() {
    cp "$tmpdir/go.mod" go.mod
    cp "$tmpdir/go.sum" go.sum
    rm -rf "$tmpdir"
  }
  trap restore RETURN

  go mod tidy

  if ! cmp -s go.mod "$tmpdir/go.mod" || ! cmp -s go.sum "$tmpdir/go.sum"; then
    printf 'go.mod or go.sum is not tidy; run go mod tidy.\n' >&2
    return 1
  fi
}

check_gofmt
check_go_mod_tidy
go vet ./...
go test -count=1 ./...

if command -v golangci-lint >/dev/null 2>&1; then
  golangci-lint run
else
  printf 'golangci-lint not found; skipping local lint. CI enforces lint.\n'
fi
