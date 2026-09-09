#!/usr/bin/env bash
set -Eeuo pipefail

if [[ -r /etc/default/cnqso-web ]]; then
    # shellcheck disable=SC1091
    source /etc/default/cnqso-web
fi

repo_dir="${CNQSO_REPO_DIR:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}"
deploy_user="${CNQSO_DEPLOY_USER:-$(stat -c '%U' "$repo_dir")}"
state_dir="${CNQSO_DEPLOY_STATE_DIR:-/var/lib/cnqso-web}"
health_url="${CNQSO_LOCAL_HEALTH_URL:-http://127.0.0.1:1739/health}"
branch="main"
remote="origin"

mkdir -p "$state_dir"
exec 9>"$state_dir/deploy.lock"
if ! flock -n 9; then
    echo "A cnqso-web deployment is already running."
    exit 0
fi

git_repo() {
    if [[ "$(id -u)" -eq 0 && "$deploy_user" != "root" ]]; then
        runuser -u "$deploy_user" -- git -C "$repo_dir" "$@"
    else
        git -C "$repo_dir" "$@"
    fi
}

if [[ "$(git_repo branch --show-current)" != "$branch" ]]; then
    echo "Refusing to deploy: $repo_dir is not on $branch." >&2
    exit 1
fi

if ! git_repo diff --cached --quiet --ignore-submodules=none || [[ -n "$(git_repo status --porcelain --untracked-files=no --ignore-submodules=all)" ]]; then
    echo "Refusing to deploy: tracked files in $repo_dir have local changes." >&2
    exit 1
fi

# A moved gitlink is expected after a partially completed update. Protect edits
# within initialized submodules separately, then reconcile their pinned revisions.
# shellcheck disable=SC2016 # Git evaluates this script inside each submodule.
git_repo submodule foreach --quiet --recursive '
    if ! git diff --cached --quiet --ignore-submodules=none || test -n "$(git status --porcelain --untracked-files=no --ignore-submodules=all)"; then
        echo "Refusing to deploy: local changes in submodule $displaypath." >&2
        exit 1
    fi
'

# Recover the existing image even if GitHub happens to be unreachable.
if ! curl --fail --silent --max-time 10 "$health_url" >/dev/null &&
   [[ -f "$state_dir/deployed-revision" ]]; then
    "$repo_dir/scripts/compose.sh" recover
fi

git_repo fetch --quiet "$remote" "$branch"
local_revision="$(git_repo rev-parse HEAD)"
remote_revision="$(git_repo rev-parse "$remote/$branch")"

if [[ "$local_revision" != "$remote_revision" ]]; then
    if ! git_repo merge-base --is-ancestor "$local_revision" "$remote_revision"; then
        echo "Refusing to deploy: local $branch has diverged from $remote/$branch." >&2
        exit 1
    fi

    echo "Updating ${local_revision:0:8} -> ${remote_revision:0:8}"
    git_repo -c submodule.recurse=false merge --ff-only "$remote_revision"
fi

git_repo submodule sync --recursive
git_repo submodule update --init --recursive

revision="$(git_repo rev-parse HEAD)"
deployed_revision="$(cat "$state_dir/deployed-revision" 2>/dev/null || true)"

if [[ "$revision" == "$deployed_revision" ]] && curl --fail --silent --max-time 10 "$health_url" >/dev/null; then
    exit 0
fi

systemctl daemon-reload
systemctl reload-or-restart cnqso-web-server.service

for attempt in {1..20}; do
    if curl --fail --silent --max-time 10 "$health_url" >/dev/null; then
        printf '%s\n' "$revision" >"$state_dir/deployed-revision.tmp"
        mv "$state_dir/deployed-revision.tmp" "$state_dir/deployed-revision"
        echo "Deployed ${revision:0:8}; health check passed."
        exit 0
    fi
    if [[ "$attempt" -lt 20 ]]; then
        sleep 2
    fi
done

echo "Deployment finished, but $health_url did not become healthy." >&2
exit 1
