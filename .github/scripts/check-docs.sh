#!/usr/bin/env bash
# Check the docs tree without needing the reference clones.
#
# Two rules from CONTRIBUTING.md and the design docs:
#   1. every doc ships in English and 中文 as a .zh-CN pair
#   2. relative links between docs resolve
#
# The heavier check, that every file:line citation in docs/ still points at a
# real line in the upstream clones, is .github/scripts/check-doc-citations.sh.
# That one needs reference/ and cannot run in CI.
set -euo pipefail

root="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$root"

python3 - "$root" <<'PY'
import io, os, re, sys

root = sys.argv[1]
docs_root = os.path.join(root, "docs")

problems = []

# Rule 1: bilingual pairs.
for dirpath, _dirnames, filenames in os.walk(docs_root):
    for name in filenames:
        if not name.endswith(".md"):
            continue
        if name.endswith(".zh-CN.md"):
            continue
        path = os.path.join(dirpath, name)
        twin = path[: -len(".md")] + ".zh-CN.md"
        if not os.path.isfile(twin):
            problems.append(("missing .zh-CN pair", os.path.relpath(path, root)))
        # English files declare the switch near the top.
        head = io.open(path, encoding="utf-8").read(800)
        if "zh-CN.md" not in head:
            problems.append(("no language switch in the first lines", os.path.relpath(path, root)))

# Rule 2: relative links resolve.
link = re.compile(r'\]\(([^)\s]+)\)')
for dirpath, _dirnames, filenames in os.walk(docs_root):
    for name in filenames:
        if not name.endswith(".md"):
            continue
        path = os.path.join(dirpath, name)
        text = io.open(path, encoding="utf-8").read()
        for match in link.finditer(text):
            target = match.group(1)
            if target.startswith(("http://", "https://", "mailto:", "#")):
                continue
            base = target.split("#", 1)[0]
            if not base:
                continue
            resolved = os.path.normpath(os.path.join(dirpath, base))
            if not os.path.exists(resolved):
                problems.append((f"broken link -> {target}", os.path.relpath(path, root)))

if problems:
    print(f"docs check: {len(problems)} problem(s)")
    for kind, where in problems:
        print(f"  {where}: {kind}")
    sys.exit(1)

print("docs check: bilingual pairs and relative links OK")
sys.exit(0)
PY