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
        "$repo_dir/scripts/prepare-data.sh"
        exec docker compose up -d --build --force-recreate web
        ;;
    recover)
        exec docker compose up -d --no-build --force-recreate web
        ;;
    down)
        exec docker compose down
        ;;
    *)
        echo "Usage: $0 {up|recover|down}" >&2
        exit 2
        ;;
esac
