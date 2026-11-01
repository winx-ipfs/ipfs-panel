#!/usr/bin/env bash
# Decide whether this workflow run publishes a GitHub Release.
# Writes notes outside the worktree and creates a local vX.Y.Z tag when it does.
set -euo pipefail

publish=false
version=""

write_output() {
  if [[ -z "${GITHUB_OUTPUT:-}" ]]; then
    return 0
  fi
  echo "publish=$1" >> "$GITHUB_OUTPUT"
  if [[ -n "${2:-}" ]]; then
    echo "version=$2" >> "$GITHUB_OUTPUT"
  fi
  if [[ -n "${3:-}" ]]; then
    echo "notes=$3" >> "$GITHUB_OUTPUT"
  fi
}

subject_ref() {
  local subject
  subject="$(git log -1 --format=%s)"
  if [[ "$subject" =~ Merge\ pull\ request\ \#[0-9]+\ from\ [^[:space:]]+/((release|hotfix)/[0-9]+\.[0-9]+\.[0-9]+) ]]; then
    printf '%s\n' "${BASH_REMATCH[1]}"
  fi
}

if [[ "${GITHUB_REF:-}" == refs/tags/v* ]]; then
  version="${GITHUB_REF#refs/tags/v}"
  publish=true
elif [[ "${GITHUB_REF:-}" == refs/heads/main ]]; then
  refs=""
  err="$(mktemp)"
  set +e
  refs="$(gh api --paginate "repos/${GITHUB_REPOSITORY}/commits/${GITHUB_SHA}/pulls" --jq '.[] | select(.base.ref=="main") | .head.ref' 2>"$err")"
  api_status=$?
  set -e
  if [[ "$api_status" -ne 0 ]]; then
    echo "release: commit/pulls lookup failed, falling back to the commit subject" >&2
    cat "$err" >&2
  fi
  rm -f "$err"
  if [[ -z "$refs" ]]; then
    fallback="$(subject_ref || true)"
    if [[ -n "$fallback" ]]; then
      refs="$fallback"
    fi
  fi
  while IFS= read -r ref; do
    [[ -z "$ref" ]] && continue
    if [[ "$ref" =~ ^(release|hotfix)/([0-9]+\.[0-9]+\.[0-9]+)$ ]]; then
      version="${BASH_REMATCH[2]}"
      publish=true
      break
    fi
  done <<< "$refs"
else
  echo "release: ref ${GITHUB_REF:-unset} is not main and not a vX.Y.Z tag"
fi

if [[ "$publish" != true ]]; then
  echo "release: nothing to publish"
  write_output false
  exit 0
fi

if [[ ! "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "release: invalid version '$version'" >&2
  exit 1
fi

if [[ ! -f CHANGELOG.md ]] || ! grep -Eq "^## \\[$version\\]( |$)" CHANGELOG.md; then
  echo "release: CHANGELOG.md is missing a heading '## [$version]'" >&2
  exit 1
fi

# Outside the worktree so GoReleaser does not see a dirty git status.
notes="${RUNNER_TEMP:-/tmp}/ipfs-panel-release-notes.md"
awk -v ver="$version" '
  $0 ~ "^## \\[" ver "\\]" { found=1; next }
  found && $0 ~ "^## \\[" { exit }
  found { print }
' CHANGELOG.md > "$notes"

if ! grep -q '[^[:space:]]' "$notes"; then
  echo "Release v$version." > "$notes"
fi

if git rev-parse -q --verify "refs/tags/v$version" >/dev/null; then
  tagged="$(git rev-parse "v$version^{}")"
  head="$(git rev-parse HEAD)"
  if [[ "$tagged" != "$head" ]]; then
    echo "release: v$version already points at $tagged, HEAD is $head" >&2
    exit 1
  fi
else
  git config user.email "41898282+github-actions[bot]@users.noreply.github.com"
  git config user.name "github-actions[bot]"
  git tag -a "v$version" -m "v$version"
fi

write_output true "$version" "$notes"
echo "release: publishing v$version"
echo "release: notes $notes"
