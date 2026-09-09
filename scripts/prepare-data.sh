#!/usr/bin/env bash
set -Eeuo pipefail
repo_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
mkdir -p "$repo_dir/go/files/public"
# Copy legacy public papers once; preserve the originals and any existing library files.
if [[ ! -e "$repo_dir/go/files/.odir-migrated" ]]; then
    if [[ -d "$repo_dir/go/odir/papers" ]]; then
        cp -an "$repo_dir/go/odir/papers" "$repo_dir/go/files/public/"
    fi
    touch "$repo_dir/go/files/.odir-migrated"
    if [[ "$(id -u)" -eq 0 ]]; then
        chown -R 1000:1000 "$repo_dir/go/files"
    fi
fi
