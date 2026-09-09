#!/usr/bin/env bash
set -Eeuo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo_dir="$(cd "$script_dir/.." && pwd)"

if [[ "$(uname -s)" != "Linux" ]]; then
    echo "This installer requires a Linux host with systemd." >&2
    exit 1
fi

if [[ "$(id -u)" -ne 0 ]]; then
    exec sudo "$0" "$@"
fi

for command in git docker systemctl flock runuser curl; do
    if ! command -v "$command" >/dev/null; then
        echo "Missing required command: $command" >&2
        exit 1
    fi
done

if ! docker compose version >/dev/null 2>&1; then
    echo "Docker Compose v2 is required." >&2
    exit 1
fi

deploy_user="$(stat -c '%U' "$repo_dir")"
if [[ "$deploy_user" == "UNKNOWN" ]]; then
    echo "Could not determine the owner of $repo_dir." >&2
    exit 1
fi

install -d -m 0755 /etc/default /var/lib/cnqso-web
{
    printf 'CNQSO_REPO_DIR=%q\n' "$repo_dir"
    printf 'CNQSO_DEPLOY_USER=%q\n' "$deploy_user"
} >/etc/default/cnqso-web
chmod 0644 /etc/default/cnqso-web

ln -sfn "$repo_dir/scripts/compose.sh" /usr/local/bin/cnqso-web-compose
ln -sfn "$repo_dir/scripts/deploy.sh" /usr/local/bin/cnqso-web-deploy
ln -sfn "$repo_dir/deploy/systemd/cnqso-web-server.service" /etc/systemd/system/cnqso-web-server.service
ln -sfn "$repo_dir/deploy/systemd/cnqso-web-deploy.service" /etc/systemd/system/cnqso-web-deploy.service
ln -sfn "$repo_dir/deploy/systemd/cnqso-web-deploy.timer" /etc/systemd/system/cnqso-web-deploy.timer

systemctl daemon-reload
systemctl enable cnqso-web-server.service cnqso-web-deploy.timer
systemctl start cnqso-web-deploy.timer
systemctl start cnqso-web-deploy.service

echo "cnqso-web auto-deployment is installed for $repo_dir."
