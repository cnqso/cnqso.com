#!/usr/bin/env bash
set -Eeuo pipefail
# Rebuild the homepage fonts with only the characters the homepage uses.
# Re-run after changing the menu. Needs a running server and: pip install fonttools brotli
repo_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
page="${1:-http://localhost:1738/}"
fonts="https://raw.githubusercontent.com/google/fonts/main/ofl"
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

text="$(curl -fsS "$page" | grep -o 'data-text="[^"]*"' | cut -d'"' -f2 | tr -d '\n')"
echo "Subsetting to: $text"

curl -fsSL -o "$work/coral-pixels.ttf" "$fonts/coralpixels/CoralPixels-Regular.ttf"
curl -fsSL -o "$work/archivo-black.ttf" "$fonts/archivoblack/ArchivoBlack-Regular.ttf"
curl -fsSL -o "$work/unbounded.ttf" "$fonts/unbounded/Unbounded%5Bwght%5D.ttf"
fonttools varLib.instancer "$work/unbounded.ttf" wght=900 -o "$work/unbounded-900.ttf"

for font in coral-pixels archivo-black unbounded-900; do
    pyftsubset "$work/$font.ttf" --text="$text" --layout-features='*' --flavor=woff2 \
        --output-file="$repo_dir/go/static/fonts/$font.woff2"
done
