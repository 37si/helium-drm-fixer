#!/bin/sh

set -eu

repo="${HDF_REPO:-37si/helium-drm-fixer}"
version="${HDF_VERSION:-}"
bindir="${HDF_BIN:-$HOME/.local/bin}"

os="$(uname -s | tr '[:upper:]' '[:lower:]')"
case "$os" in
  darwin) os=darwin ;;
  linux) os=linux ;;
  *) echo "unsupported system: $os" >&2; exit 1 ;;
esac

arch="$(uname -m)"
case "$arch" in
  x86_64|amd64) arch=amd64 ;;
  arm64|aarch64) arch=arm64 ;;
  *) echo "unsupported architecture: $arch" >&2; exit 1 ;;
esac

if [ -z "$version" ]; then
  version="$(curl -fsSL "https://api.github.com/repos/$repo/releases/latest" |
    sed -n 's/.*"tag_name": *"v\{0,1\}\([^"]*\)".*/\1/p' | head -n 1)"
fi

if [ -z "$version" ]; then
  echo "could not work out the newest release of $repo" >&2
  exit 1
fi

name="hdf_${version}_${os}_${arch}"
base="https://github.com/$repo/releases/download/v$version"

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

echo "downloading $name"
curl -fsSL "$base/$name" -o "$tmp/hdf"
curl -fsSL "$base/sha256.txt" -o "$tmp/sha256.txt"

expected="$(awk -v name="$name" '
  {
    file = $2
    sub(/^\*/, "", file)
    sub(/\r$/, "", file)
    if (file == name) {
      print $1
      exit
    }
  }
' "$tmp/sha256.txt")"
if [ -z "$expected" ]; then
  echo "sha256.txt has no entry for $name" >&2
  exit 1
fi

if command -v sha256sum >/dev/null 2>&1; then
  actual="$(sha256sum "$tmp/hdf" | awk '{ print $1 }')"
elif command -v shasum >/dev/null 2>&1; then
  actual="$(shasum -a 256 "$tmp/hdf" | awk '{ print $1 }')"
else
  echo "need sha256sum or shasum to verify the download" >&2
  exit 1
fi

if [ "$expected" != "$actual" ]; then
  echo "checksum mismatch for $name" >&2
  exit 1
fi

mkdir -p "$bindir"
install -m 0755 "$tmp/hdf" "$bindir/hdf"

echo "installed $bindir/hdf ($version)"
case ":$PATH:" in
  *":$bindir:"*) ;;
  *) echo "add $bindir to PATH to run it from anywhere" ;;
esac
