#!/usr/bin/env bash
# Coverage gate: fails unless the packages that the project policy requires to
# be fully tested reach 100 % statement coverage. A package without
# statements (only a package comment) counts as fully covered.
set -euo pipefail

readonly REQUIRED_PACKAGES=(
  ./internal/core/...
  ./internal/adapter/publicapi/v1/...
  ./internal/adapter/admin/...
)

profile="$(mktemp)"
trap 'rm -f "$profile"' EXIT

go test -covermode=set -coverprofile="$profile" "${REQUIRED_PACKAGES[@]}"

# Profile lines after the "mode:" header look like
# "file.go:line.col,line.col numStatements hitCount".
awk '
  NR == 1 { next }
  {
    total += $2
    if ($3 > 0) { covered += $2 } else { print "uncovered: " $1 }
  }
  END {
    if (total == 0) { print "coverage gate: no statements yet, passing"; exit 0 }
    printf "coverage gate: %d of %d statements covered\n", covered, total
    if (covered < total) { print "coverage gate: 100 % required"; exit 1 }
  }
' "$profile"
