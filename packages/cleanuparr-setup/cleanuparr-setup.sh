# shellcheck shell=bash
# Complete Cleanuparr's first-run setup and export its API key.
#
# Cleanuparr mints its API key with the admin account during first-run setup and
# only serves it from the API — there is no config file or DB row to preseed, and
# every non-auth /api/* path 403s "Setup required" until setup completes.
#
# Idempotent: /api/auth/status is unauthenticated and the account lives in
# PostgreSQL rather than the ephemeral rootfs, so a reboot re-runs this harmlessly.
#
# Credentials are read from the environment (CLEANUPARR_ADMIN_USER /
# CLEANUPARR_ADMIN_PASSWORD) and reach curl through a pipe, never argv — argv is
# world-readable in /proc.
#
# Args:
#   --base URL        Cleanuparr base URL (http://127.0.0.1:11011)
#   --key-file PATH   where to write CLEANUPARR_API_KEY=…
#                     (/run/cleanuparr/api-key.env)

base=http://127.0.0.1:11011
key_file=/run/cleanuparr/api-key.env

while [ "$#" -gt 0 ]; do
  case "$1" in
  --base)
    base="$2"
    shift 2
    ;;
  --key-file)
    key_file="$2"
    shift 2
    ;;
  *)
    echo "unknown argument: $1" >&2
    exit 2
    ;;
  esac
done

creds() {
  jq -nc '{username: env.CLEANUPARR_ADMIN_USER, password: env.CLEANUPARR_ADMIN_PASSWORD}'
}

until curl -sf "$base/health" >/dev/null; do sleep 1; done

if [ "$(curl -sf "$base/api/auth/status" | jq -r .setupCompleted)" != "true" ]; then
  creds |
    curl -sf -X POST "$base/api/auth/setup/account" \
      -H 'Content-Type: application/json' --data-binary @- >/dev/null
  curl -sf -X POST "$base/api/auth/setup/complete" >/dev/null
fi

jwt=$(creds |
  curl -sf -X POST "$base/api/auth/login" \
    -H 'Content-Type: application/json' --data-binary @- |
  jq -r '.tokens.accessToken')
key=$(curl -sf "$base/api/account/api-key" -H "Authorization: Bearer $jwt" | jq -r .apiKey)

umask 077
printf 'CLEANUPARR_API_KEY=%s\n' "$key" >"$key_file"
