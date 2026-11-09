#!/usr/bin/env bash
# Local test for check-pr-body.sh. Not run in CI: it exists so the checker has
# a passing case and three failing cases, proving it is neither inert nor
# always-failing.
set -uo pipefail

root="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$root"

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

pass=0
fail=0

check() {
  local want="$1" name="$2" body="$3"
  PR_BODY="$body" bash .github/scripts/check-pr-body.sh >"$tmp/out" 2>&1
  local got=$?
  if [[ "$got" == "$want" ]]; then
    pass=$((pass + 1))
    printf 'ok   %-40s exit=%s\n' "$name" "$got"
  else
    fail=$((fail + 1))
    printf 'FAIL %-40s expected %s, got %s\n' "$name" "$want" "$got"
    sed -n '1,6p' "$tmp/out"
  fi
}

check 1 "empty body" ""
check 1 "section absent" "## What

Only a problem statement."

check 1 "heading with no evidence" "## Verification

- [ ] tests pass
<!-- nothing written -->"

check 0 "heading with commands and output" "## Verification

\`go test ./...\` -> ok all packages
\`curl /api/node\` -> {\"state\":\"down\"}"

check 0 "evidence before a deeper heading" "## Verification

\`npm test\` -> 46 passed

### Notes

Nothing else."

echo
echo "passed $pass, failed $fail"
[[ "$fail" -eq 0 ]]