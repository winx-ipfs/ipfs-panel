#!/usr/bin/env bash
# Download the one pinned Kubo release and verify its published checksum.
#
# The pin is a released binary, not a git commit. Change the version only in
# a pull request that says why, and update TestRealKuboOptional in the same
# change: the test rejects a release it has no settled expectations for.
#
# CI runs this for linux/amd64, which is the runner the harness starts on.
# Other OS/arch pairs are rejected here until a platform is added on purpose.
#
#   fetch-kubo.sh <dest>
#       downloads into <dest>, writes <dest>/kubo-version, and prints the
#       directory that contains the ipfs binary.
#
#   fetch-kubo.sh --check <file> <sha512-hex>
#       exits 0 when the file's sha512 matches, 1 otherwise.
#
#   fetch-kubo.sh --pin-matches <sumfile> <sha512-hex>
#       exits 0 when the first field of the published checksum file equals
#       the pin. A published file that does not match the pin is a failure.
set -euo pipefail

version=v0.43.1
# No leading "v". This is the string Kubo puts in its agent version.
version_number=0.43.1
# Published sha512 of kubo_v0.43.1_linux-amd64.tar.gz on the GitHub release
# (2026-09-15). sha512sum writes the hash, two spaces, then the filename.
pin_sha512=ff53b2428794fc8cca39505d28c15b4cceaef4b90a09284f291f611fcdc8c06399690575fc89cbd4d85ef71f3652581296c6f2e710386f887c8edf36b6e89d71

file_sha512() {
  sha512sum "$1" | awk '{print $1}'
}

published_sha512() {
  awk 'NF { print $1; exit }' "$1"
}

if [[ "${1:-}" == "--check" ]]; then
  file="${2:-}"
  expect="${3:-}"
  if [[ -z "$file" || -z "$expect" || ! -f "$file" ]]; then
    echo "fetch-kubo: --check needs a file and a sha512" >&2
    exit 1
  fi
  got="$(file_sha512 "$file")"
  if [[ "$got" != "$expect" ]]; then
    echo "fetch-kubo: checksum mismatch" >&2
    echo "  file:   $file" >&2
    echo "  got:    $got" >&2
    echo "  expect: $expect" >&2
    exit 1
  fi
  exit 0
fi

if [[ "${1:-}" == "--pin-matches" ]]; then
  sumfile="${2:-}"
  expect="${3:-}"
  if [[ -z "$sumfile" || -z "$expect" || ! -f "$sumfile" ]]; then
    echo "fetch-kubo: --pin-matches needs a checksum file and a sha512" >&2
    exit 1
  fi
  got="$(published_sha512 "$sumfile")"
  if [[ -z "$got" || "$got" != "$expect" ]]; then
    echo "fetch-kubo: published checksum does not match the pin" >&2
    echo "  file:   $sumfile" >&2
    echo "  got:    ${got:-<empty>}" >&2
    echo "  expect: $expect" >&2
    exit 1
  fi
  exit 0
fi

dest="${1:-}"
if [[ -z "$dest" ]]; then
  echo "fetch-kubo: destination directory is required" >&2
  exit 1
fi

os="$(uname -s | tr '[:upper:]' '[:lower:]')"
arch="$(uname -m)"
if [[ "$os" != "linux" || "$arch" != "x86_64" ]]; then
  echo "fetch-kubo: the pin is linux/amd64 only (this host is ${os}/${arch})" >&2
  echo "fetch-kubo: the harness starts on the ubuntu runner CI already uses" >&2
  exit 1
fi

asset="kubo_${version}_linux-amd64.tar.gz"
url="https://github.com/ipfs/kubo/releases/download/${version}/${asset}"
mkdir -p "$dest"
archive="$dest/$asset"
sumfile="$dest/${asset}.sha512"

curl -fsSL --retry 5 --retry-delay 2 -o "$archive" "$url"
curl -fsSL --retry 5 --retry-delay 2 -o "$sumfile" "${url}.sha512"

bash "$0" --pin-matches "$sumfile" "$pin_sha512"
bash "$0" --check "$archive" "$pin_sha512"

tar -xzf "$archive" -C "$dest"
bin="$dest/kubo/ipfs"
if [[ ! -x "$bin" ]]; then
  echo "fetch-kubo: archive has no kubo/ipfs" >&2
  exit 1
fi
printf '%s\n' "$version_number" > "$dest/kubo-version"
printf '%s\n' "$dest/kubo"
