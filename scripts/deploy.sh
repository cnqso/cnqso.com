#!/usr/bin/env bash
set -Eeuo pipefail

APP_DIR="${CNQSO_DEPLOY_DIR:-/home/asendio/cnqso-web}"
LOCK_FILE="${CNQSO_DEPLOY_LOCK:-/tmp/cnqso-web-deploy.lock}"
LOCAL_HEALTH_URL="${CNQSO_LOCAL_HEALTH_URL:-http://127.0.0.1:1739/health}"
PUBLIC_HEALTH_URL="${CNQSO_PUBLIC_HEALTH_URL:-https://cnqso.com/health}"

wait_for_url() {
	local name="$1"
	local url="$2"
	local attempts="${3:-12}"
	local delay="${4:-2}"

	for ((attempt = 1; attempt <= attempts; attempt++)); do
		if curl --fail --silent --show-error --max-time 10 "$url" >/dev/null; then
			echo "$name health check passed"
			return 0
		fi

		echo "$name health check failed on attempt $attempt/$attempts"
		sleep "$delay"
	done

	echo "$name health check failed after $attempts attempts: $url"
	return 1
}

wait_for_container_health() {
	local container="$1"
	local attempts="${2:-20}"
	local delay="${3:-2}"
	local status

	for ((attempt = 1; attempt <= attempts; attempt++)); do
		status="$(sudo -n docker inspect --format '{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}' "$container")"
		if [ "$status" = "healthy" ] || [ "$status" = "running" ]; then
			echo "$container container status is $status"
			return 0
		fi

		echo "$container container status is $status on attempt $attempt/$attempts"
		sleep "$delay"
	done

	echo "$container did not become healthy after $attempts attempts"
	return 1
}

cd "$APP_DIR"

exec 9>"$LOCK_FILE"
if ! flock -n 9; then
	echo "Another cnqso-web deploy is already running."
	exit 1
fi

echo "Starting cnqso-web deploy in $APP_DIR"
echo "Current revision: $(git rev-parse --short HEAD)"

git fetch origin main
git checkout main
git pull --ff-only origin main

echo "Deploying revision: $(git rev-parse --short HEAD)"

/usr/bin/time -f "compose_elapsed=%E compose_user=%U compose_sys=%S compose_maxrss_kb=%M" \
	sudo -n docker compose up -d --build

wait_for_url "local" "$LOCAL_HEALTH_URL"
wait_for_url "public" "$PUBLIC_HEALTH_URL"
wait_for_container_health "cnqso-web-server"

sudo -n docker ps --filter name=cnqso-web-server --format "container={{.Names}} status={{.Status}} ports={{.Ports}}"
echo "Deploy complete: $(git rev-parse --short HEAD)"
