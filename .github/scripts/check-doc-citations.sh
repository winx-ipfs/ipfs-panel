#!/usr/bin/env bash
# Check every file:line citation in docs/ against the reference clones.
#
# The docs promise that a claim without a citation is a claim that was not
# verified, and that every citation resolves. This script enforces the second
# half. Run it after editing any doc under docs/, and after moving a pin in
# fetch-reference.sh.
#
# Exits non-zero when a cited file is missing or a cited line is past the end
# of the file. Both mean the doc is pointing at something that is not there,
# which is worse than no citation at all.
set -euo pipefail

root="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$root"

if [[ ! -d reference ]]; then
  echo "reference/ is missing. Run .github/scripts/fetch-reference.sh first." >&2
  exit 2
fi

python3 - "$root" <<'PY'
import io, os, re, sys

root = sys.argv[1]
ref = os.path.join(root, "reference")
docs = [
    "docs/REFERENCE-ipfs-stack.md",
    "docs/REFERENCE-ipfs-stack.zh-CN.md",
    "docs/design/harness.md",
    "docs/design/harness.zh-CN.md",
    "docs/design/kubo-integration.md",
    "docs/design/kubo-integration.zh-CN.md",
    "docs/design/scenes.md",
    "docs/design/scenes.zh-CN.md",
    "docs/design/webui-parity.md",
    "docs/design/webui-parity.zh-CN.md",
    "docs/design/explore.md",
    "docs/design/explore.zh-CN.md",
    "docs/DESIGN-node-lifecycle.md",
    "docs/DESIGN-node-lifecycle.zh-CN.md",
]
# file.ext:LINE or file.ext:START-END
cite = re.compile(r'([\w./-]+\.(?:go|md|sh|tsx|ts|jsx|js|json|yml))\s*:\s*(\d+)(?:-(\d+))?')


def resolve(path):
    for base in (ref, os.path.join(root, "services", "manager"), root):
        candidate = os.path.join(base, path)
        if os.path.isfile(candidate):
            return candidate
    for repo in ("kubo", "boxo", "helia", "ipfs-docs", "specs"):
        candidate = os.path.join(ref, repo, path)
        if os.path.isfile(candidate):
            return candidate
    return None


checked = 0
missing = []
past_end = []

for doc in docs:
    full = os.path.join(root, doc)
    if not os.path.isfile(full):
        continue
    text = io.open(full, encoding="utf-8").read()
    for match in cite.finditer(text):
        path, start, end = match.group(1), int(match.group(2)), match.group(3)
        target = resolve(path)
        if target is None:
            missing.append((doc, path))
            continue
        total = sum(1 for _ in io.open(target, encoding="utf-8", errors="replace"))
        checked += 1
        limit = int(end) if end else start
        if limit > total:
            past_end.append((doc, path, limit, total))

print(f"citations checked: {checked}")
if missing:
    print(f"\nunresolved ({len(missing)}):")
    for doc, path in sorted(set(missing)):
        print(f"  {doc}: {path}")
if past_end:
    print(f"\npast end of file ({len(past_end)}):")
    for doc, path, limit, total in past_end:
        print(f"  {doc}: {path}:{limit} but the file has {total} lines")

sys.exit(1 if (missing or past_end) else 0)
PY