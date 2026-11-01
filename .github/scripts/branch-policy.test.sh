#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "$0")/../.." && pwd)"
script="$root/.github/scripts/branch-policy.sh"
pass=0
fail=0

expect() {
  local want="$1"
  local name="$2"
  local dir="$3"
  shift 3
  local output actual
  set +e
  output="$(cd "$dir" && env "$@" "$script" 2>&1)"
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

cd "$root"
export GITHUB_EVENT_NAME=pull_request

expect 0 "feature into develop" "$root" BASE_REF=develop HEAD_REF=feature/shell
expect 0 "fix into develop" "$root" BASE_REF=develop HEAD_REF=fix/health-json
expect 0 "chore into develop" "$root" BASE_REF=develop HEAD_REF=chore/governance
expect 0 "docs into develop" "$root" BASE_REF=develop HEAD_REF=docs/vision
expect 0 "refactor into develop" "$root" BASE_REF=develop HEAD_REF=refactor/mux
expect 0 "test into develop" "$root" BASE_REF=develop HEAD_REF=test/locales
expect 0 "ci into develop" "$root" BASE_REF=develop HEAD_REF=ci/nightly
expect 0 "release into develop" "$root" BASE_REF=develop HEAD_REF=release/1.2.3
expect 0 "hotfix into develop" "$root" BASE_REF=develop HEAD_REF=hotfix/1.2.4
expect 0 "dependabot into develop" "$root" BASE_REF=develop HEAD_REF=dependabot/npm_and_yarn/vite-1.2.3
expect 0 "back-merge into develop" "$root" BASE_REF=develop HEAD_REF=main
expect 1 "uppercase feature" "$root" BASE_REF=develop HEAD_REF=Feature/shell
expect 1 "feat prefix rejected" "$root" BASE_REF=develop HEAD_REF=feat/shell
expect 1 "cursor prefix into develop" "$root" BASE_REF=develop HEAD_REF=cursor/governance-ci-0ac7
expect 1 "release missing changelog" "$root" BASE_REF=main HEAD_REF=release/9.9.9 DEVELOP_EXISTS=yes
expect 1 "hotfix missing changelog" "$root" BASE_REF=main HEAD_REF=hotfix/9.9.9 DEVELOP_EXISTS=yes
expect 1 "v prefix in branch name" "$root" BASE_REF=main HEAD_REF=release/v1.2.3 DEVELOP_EXISTS=yes
expect 1 "short version" "$root" BASE_REF=main HEAD_REF=hotfix/1.2 DEVELOP_EXISTS=yes
expect 1 "feature into main" "$root" BASE_REF=main HEAD_REF=feature/shell DEVELOP_EXISTS=yes
expect 1 "main into main" "$root" BASE_REF=main HEAD_REF=main DEVELOP_EXISTS=yes
expect 0 "bootstrap before develop exists" "$root" BASE_REF=main HEAD_REF=cursor/governance-ci-0ac7 DEVELOP_EXISTS=no
expect 0 "push events are ignored" "$root" GITHUB_EVENT_NAME=push BASE_REF=main HEAD_REF=feature/shell DEVELOP_EXISTS=yes

work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
printf '%s\n' '# Changelog' '' '## [1.2.3] - 2026-11-01' '' '- ship it' '' > "$work/CHANGELOG.md"
expect 0 "release with changelog" "$work" \
  GITHUB_EVENT_NAME=pull_request \
  BASE_REF=main \
  HEAD_REF=release/1.2.3 \
  DEVELOP_EXISTS=yes
expect 0 "hotfix with changelog" "$work" \
  GITHUB_EVENT_NAME=pull_request \
  BASE_REF=main \
  HEAD_REF=hotfix/1.2.3 \
  DEVELOP_EXISTS=yes

echo "$pass passed, $fail failed"
if [[ "$fail" -ne 0 ]]; then
  exit 1
fi
