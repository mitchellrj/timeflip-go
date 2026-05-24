#!/usr/bin/env bash
set -euo pipefail

root="$(git rev-parse --show-toplevel)"
cd "$root"

find scripts -name '*.sh' -print0 | xargs -0 -n1 bash -n
