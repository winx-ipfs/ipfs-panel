#!/usr/bin/env bash
# Build apps/web and attach the bundle plus its sha256 to the GitHub Release.
set -euo pipefail

if [[ -z "${VERSION:-}" ]]; then
  echo "upload-web-dist: VERSION is required" >&2
  exit 1
fi

root="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$root/apps/web"
npm ci
npm run build

mkdir -p "$root/dist"
archive="$root/dist/ipfs-panel-web_${VERSION}_dist.zip"
checksum="${archive}.sha256"
rm -f "$archive" "$checksum"
(
  cd dist
  zip -r "$archive" .
)
(
  cd "$root/dist"
  sha256sum "$(basename "$archive")" > "$(basename "$checksum")"
)
gh release upload "v$VERSION" "$archive" "$checksum" --clobber
echo "upload-web-dist: attached $(basename "$archive")"
