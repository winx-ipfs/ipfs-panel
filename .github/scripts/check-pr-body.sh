#!/usr/bin/env bash
# Require a pull request to say what was actually run.
#
# AGENTS.md makes three things mandatory before a pull request is reviewable:
# the product runs, the suite is green, and the printed claim matches observed
# behaviour. This script enforces the weakest form of that: the description
# contains a Verification section with at least one non-empty line of evidence.
#
# It cannot check whether the evidence is true. That is review's job. It can
# check that the field is not empty, which is the failure that actually happens:
# a pull request with an untouched template.
set -euo pipefail

body="${PR_BODY:-}"

if [[ -z "${body//[[:space:]]/}" ]]; then
  echo "pull request body is empty; fill the template, including Verification" >&2
  exit 1
fi

# Find the Verification heading and read until the next heading of the same or
# higher level. Accept "#", "##", or "###".
section="$(
  printf '%s\n' "$body" | awk '
    /^#{1,3}[[:space:]]*Verification/ { capture = 1; level = length($1); next }
    capture && /^#{1,3}[[:space:]]/ {
      if (length($1) <= level) capture = 0
    }
    capture { print }
  '
)"

# Strip HTML comments, checkbox syntax, and whitespace. What remains is what the
# author actually wrote.
evidence="$(
  printf '%s\n' "$section" \
    | sed -E 's/<!--.*-->//g; s/^[[:space:]]*//; s/[[:space:]]*$//' \
    | grep -vE '^$' \
    | grep -vE '^[-*][[:space:]]+\[ \]' \
    || true
)"

if [[ -z "${evidence//[[:space:]]/}" ]]; then
  cat >&2 <<'EOF'
The Verification section is missing or empty.

AGENTS.md requires three things, in this order:

  1. the product runs            (start it, exercise the changed path, paste the output)
  2. the suite is green          (the exact commands from README.md)
  3. the printed claim matches   (if you say a stopped node returns node_not_running,
                                  run it while stopped and show the response)

Add a "## Verification" heading and put the commands and their output under it.
A green test suite alone is not step 1.
EOF
  exit 1
fi

echo "Verification section present:"
printf '%s\n' "$evidence" | head -20