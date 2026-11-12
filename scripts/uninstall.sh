#!/bin/sh

set -eu

bindir="${HDF_BIN:-$HOME/.local/bin}"
purge="${HDF_PURGE:-}"

for arg in "$@"; do
  case "$arg" in
    --purge) purge=1 ;;
    -h|--help)
      echo "usage: uninstall.sh [--purge]"
      exit 0
      ;;
  esac
done

binary="$bindir/hdf"
if [ -f "$binary" ]; then
  rm -f "$binary"
  echo "removed $binary"
else
  echo "$binary was not there"
fi

case "$(uname -s)" in
  Darwin)
    config="$HOME/Library/Application Support/hdf"
    cache="$HOME/Library/Caches/hdf"
    ;;
  *)
    config="${XDG_CONFIG_HOME:-$HOME/.config}/hdf"
    cache="${XDG_CACHE_HOME:-$HOME/.cache}/hdf"
    ;;
esac

case "$purge" in
  1|true|yes)
    rm -rf "$config" "$cache"
    echo "removed $config and $cache"
    ;;
  *)
    echo "kept settings and backups in $config"
    echo "run with --purge or HDF_PURGE=1 to remove them too"
    ;;
esac
