#!/usr/bin/env bash
# Fetch the upstream sources this repository's docs cite.
#
# The clones land in reference/, which .gitignore excludes. They are study
# material for a reader or an AI agent, never part of the product, and never
# committed: the code belongs to the IPFS project under its own licenses.
#
# The commits below are the ones every file:line citation in docs/ was checked
# against. Fetching a newer commit will move those lines. If you refresh them,
# re-run the citation check described in docs/REFERENCE-ipfs-stack.md and
# update the table there.
set -euo pipefail

root="$(cd "$(dirname "$0")/../.." && pwd)"
ref="$root/reference"
mkdir -p "$ref"

# repo=commit=date. Keep in step with docs/REFERENCE-ipfs-stack.md.
repos='
kubo=833e04a240d8c533509ddaaa3c202b39b0bb3024
boxo=3d6ac39a0f76c824ffbb90e3a729e78f62bc2d12
helia=cba1ed69b4369924790d9eccdb8c9089435f4f20
ipfs-docs=1261c016f2b6c700e9354e46cedcdf38c9ad7b91
specs=3000be69497e47c399cdb719440d0809f085388b
ipfs-webui=c51d980920d60e9d41f53f461886d6e1ea046298
'

fetch() {
  local name="$1" commit="$2" dest="$ref/$name"
  if [[ -d "$dest/.git" ]]; then
    if git -C "$dest" cat-file -e "$commit^{commit}" 2>/dev/null; then
      printf '%-10s already at %s\n' "$name" "${commit:0:10}"
      return 0
    fi
    echo "$name: fetching $commit"
    git -C "$dest" fetch --depth 1 origin "$commit"
    git -C "$dest" checkout -q FETCH_HEAD
    return 0
  fi
  echo "$name: cloning $commit"
  git init -q "$dest"
  git -C "$dest" remote add origin "https://github.com/ipfs/$name.git"
  git -C "$dest" fetch --depth 1 -q origin "$commit"
  git -C "$dest" checkout -q FETCH_HEAD
}

while IFS='=' read -r name commit; do
  [[ -z "$name" ]] && continue
  fetch "$name" "$commit"
done <<< "$repos"

echo
echo "reference/ is excluded from git; nothing here is committed."
for d in "$ref"/*/; do
  [[ -d "$d/.git" ]] || continue
  printf '%-10s %s\n' "$(basename "$d")" "$(git -C "$d" rev-parse --short=10 HEAD)"
done