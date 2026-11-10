#!/usr/bin/env bash
# The checksum gate has to be able to fail. This injects a bad file, a bad
# published checksum, and a missing file, and expects each one to exit 1.
set -euo pipefail

root="$(cd "$(dirname "$0")/../.." && pwd)"
script="$root/.github/scripts/fetch-kubo.sh"
pass=0
fail=0

expect() {
  local want="$1"
  local name="$2"
  shift 2
  local output actual
  set +e
  output="$("$script" "$@" 2>&1)"
  actual=$?
  set -e
  if [[ "$actual" -eq "$want" ]]; then
    pass=$((pass + 1))
    echo "ok  $name"
  else
    fail=$((fail + 1))
    echo "FAIL $name (expected $want, got $actual)"
    echo "$output"
  fi
}

work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

printf 'hello' > "$work/file"
good="$(sha512sum "$work/file" | awk '{print $1}')"

expect 0 "matching file" --check "$work/file" "$good"
expect 1 "mismatched file" --check "$work/file" "deadbeef"
expect 1 "missing file" --check "$work/missing" "$good"
expect 1 "check without args" --check

printf '%s  kubo.tar.gz\n' "$good" > "$work/ok.sha512"
printf 'deadbeef  kubo.tar.gz\n' > "$work/bad.sha512"
: > "$work/empty.sha512"

expect 0 "published checksum matches the pin" --pin-matches "$work/ok.sha512" "$good"
expect 1 "published checksum differs from the pin" --pin-matches "$work/bad.sha512" "$good"
expect 1 "empty published checksum" --pin-matches "$work/empty.sha512" "$good"
expect 1 "pin match without a file" --pin-matches "$work/missing" "$good"

echo "$pass passed, $fail failed"
if [[ "$fail" -ne 0 ]]; then
  exit 1
fi
