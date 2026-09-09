#!/usr/bin/env bash
set -Eeuo pipefail

if [[ -r /etc/default/cnqso-web ]]; then
    # shellcheck disable=SC1091
    source /etc/default/cnqso-web
fi

repo_dir="${CNQSO_REPO_DIR:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}"
action="${1:-up}"

cd "$repo_dir"

case "$action" in
    up)
        exec docker compose up -d --build
        ;;
    down)
        exec docker compose down
        ;;
    *)
        echo "Usage: $0 {up|down}" >&2
        exit 2
        ;;
esac
