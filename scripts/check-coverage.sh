#!/usr/bin/env bash
# Coverage gate: fails unless the packages that the project policy requires to
# be fully tested reach 100 % statement coverage. A package without
# statements (only a package comment) counts as fully covered. Generated
# files (header "// Code generated ... DO NOT EDIT.") do not count.
set -euo pipefail

readonly REQUIRED_PACKAGES=(
  ./internal/core/...
  ./internal/adapter/publicapi/v1/...
  ./internal/adapter/admin/...
)

# Go's convention for the header that marks a generated file; it counts
# only before the package clause.
readonly GENERATED_HEADER='^// Code generated .* DO NOT EDIT\.'
export GENERATED_HEADER

profile="$(mktemp)"
generated="$(mktemp)"
trap 'rm -f "$profile" "$generated"' EXIT

go test -covermode=set -coverprofile="$profile" "${REQUIRED_PACKAGES[@]}"

# Profile paths start with the module path; the file lies below the module
# root at the rest of the path.
module="$(go list -m)"
tail -n +2 "$profile" | cut -d: -f1 | sort -u | while read -r file; do
  # ENVIRON keeps the backslash of the pattern, which awk -v would consume.
  if awk '
    /^package / { exit }
    $0 ~ ENVIRON["GENERATED_HEADER"] { found = 1; exit }
    END { exit !found }
  ' "${file#"$module/"}"; then
    echo "$file"
  fi
done >"$generated"

# Profile lines after the "mode:" header look like
# "file.go:line.col,line.col numStatements hitCount".
awk -v generated_list="$generated" '
  BEGIN { while ((getline file < generated_list) > 0) { generated[file] = 1 } }
  NR == 1 { next }
  {
    split($1, location, ":")
    if (location[1] in generated) { next }
    total += $2
    if ($3 > 0) { covered += $2 } else { print "uncovered: " $1 }
  }
  END {
    if (total == 0) { print "coverage gate: no statements yet, passing"; exit 0 }
    printf "coverage gate: %d of %d statements covered\n", covered, total
    if (covered < total) { print "coverage gate: 100 % required"; exit 1 }
  }
' "$profile"
