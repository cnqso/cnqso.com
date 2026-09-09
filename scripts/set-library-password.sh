#!/usr/bin/env bash
set -Eeuo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/.."
IFS= read -r -s -p "New library password (12–72 bytes): " library_password
echo
IFS= read -r -s -p "Repeat password: " library_confirmation
echo
if [[ "$library_password" != "$library_confirmation" ]]; then
    echo "Passwords do not match." >&2
    exit 1
fi
# Plaintext travels only through stdin; the helper stores a salted hash.
printf '%s\n' "$library_password" | docker compose run --rm -T --no-deps web ./library-password
unset library_password library_confirmation
