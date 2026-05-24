#!/usr/bin/env bash
set -euo pipefail

root="$(git rev-parse --show-toplevel)"
cd "$root"

go test -run='^$' -fuzz=FuzzDecodeProtocolPayloads -fuzztime="${GO_FUZZTIME:-10s}" .
