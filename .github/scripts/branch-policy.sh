#!/usr/bin/env bash
# Git Flow pull-request targets.
# PRs into main must come from release/X.Y.Z or hotfix/X.Y.Z once origin/develop exists.
# Back-merges are main -> develop and are allowed there, not into main.
# Until origin/develop exists, this bootstrap window allows other heads into main
# so the governance PR can land. A misnamed release/* or hotfix/* still fails.
set -euo pipefail

base="${BASE_REF:-}"
head="${HEAD_REF:-}"
event="${GITHUB_EVENT_NAME:-pull_request}"

if [[ "$event" != "pull_request" ]]; then
  echo "branch-policy: not a pull request ($event)"
  exit 0
fi

if [[ -z "$base" || -z "$head" ]]; then
  echo "branch-policy: BASE_REF and HEAD_REF are required" >&2
  exit 1
fi

develop_head_ok() {
  case "$head" in
    feature/*|fix/*|chore/*|docs/*|refactor/*|test/*|ci/*|release/*|hotfix/*|dependabot/*|main)
      return 0
      ;;
  esac
  return 1
}

if [[ "$base" == "develop" ]]; then
  if develop_head_ok; then
    echo "branch-policy: $head -> develop is allowed"
    exit 0
  fi
  cat >&2 <<EOF
branch-policy: '$head' cannot target develop.
Use a lowercase prefix: feature/, fix/, chore/, docs/, refactor/, test/, ci/, release/, hotfix/, dependabot/, or main for a back-merge.
EOF
  exit 1
fi

if [[ "$base" != "main" ]]; then
  echo "branch-policy: no extra rules for base $base"
  exit 0
fi

if [[ "$head" =~ ^(release|hotfix)/([0-9]+\.[0-9]+\.[0-9]+)$ ]]; then
  version="${BASH_REMATCH[2]}"
  if [[ ! -f CHANGELOG.md ]] || ! grep -Eq "^## \\[$version\\]( |$)" CHANGELOG.md; then
    echo "branch-policy: CHANGELOG.md must contain a heading '## [$version]' before merging to main" >&2
    exit 1
  fi
  echo "branch-policy: $head -> main is allowed"
  exit 0
fi

if [[ "$head" == release/* || "$head" == hotfix/* ]]; then
  echo "branch-policy: $head must be release/X.Y.Z or hotfix/X.Y.Z (numeric version, no v prefix)" >&2
  exit 1
fi

develop_exists=""
case "${DEVELOP_EXISTS:-}" in
  yes) develop_exists=1 ;;
  no) develop_exists=0 ;;
  "")
    if git ls-remote --heads origin develop | grep -q .; then
      develop_exists=1
    else
      develop_exists=0
    fi
    ;;
  *)
    echo "branch-policy: DEVELOP_EXISTS must be yes, no, or empty" >&2
    exit 1
    ;;
esac

if [[ "$develop_exists" -eq 0 ]]; then
  echo "branch-policy: origin/develop does not exist yet; this bootstrap pull request into main is allowed"
  exit 0
fi

cat >&2 <<EOF
branch-policy: pull requests into main must come from release/X.Y.Z or hotfix/X.Y.Z.
Back-merges go the other way: open them with base develop and head main.
EOF
exit 1
