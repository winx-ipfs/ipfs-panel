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

# Markdown outside docs/ that still ships as a pair with a language switch.
EXTRA = ["AGENTS.md"]

problems = []


def check_pair(path, shown):
    twin = path[: -len(".md")] + ".zh-CN.md"
    if not os.path.isfile(twin):
        problems.append(("missing .zh-CN pair", shown))
    head = io.open(path, encoding="utf-8").read(800)
    if "zh-CN.md" not in head:
        problems.append(("no language switch in the first lines", shown))


# Rule 1: bilingual pairs, for docs/ and for EXTRA.
for dirpath, _dirnames, filenames in os.walk(docs_root):
    for name in filenames:
        if not name.endswith(".md") or name.endswith(".zh-CN.md"):
            continue
        path = os.path.join(dirpath, name)
        check_pair(path, os.path.relpath(path, root))

for rel in EXTRA:
    path = os.path.join(root, rel)
    if not os.path.isfile(path):
        problems.append(("missing paired doc", rel))
        continue
    check_pair(path, rel)

# Rule 2: relative links resolve, across everything scanned above plus the
# Chinese halves of the EXTRA files.
link = re.compile(r'\]\(([^)\s]+)\)')

targets = []
for dirpath, _dirnames, filenames in os.walk(docs_root):
    for name in filenames:
        if name.endswith(".md"):
            targets.append(os.path.join(dirpath, name))
for rel in EXTRA:
    targets.append(os.path.join(root, rel))
    targets.append(os.path.join(root, rel[: -len(".md")] + ".zh-CN.md"))

for path in targets:
    if not os.path.isfile(path):
        continue
    text = io.open(path, encoding="utf-8").read()
    for match in link.finditer(text):
        target = match.group(1)
        if target.startswith(("http://", "https://", "mailto:", "#")):
            continue
        base = target.split("#", 1)[0]
        if not base:
            continue
        resolved = os.path.normpath(os.path.join(os.path.dirname(path), base))
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