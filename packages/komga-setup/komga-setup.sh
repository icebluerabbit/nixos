# shellcheck shell=bash
# Claim a fresh Komga server and export an API key for downstream consumers
# (configuratarr).
#
# Komga has no environment variable that seeds the initial admin — its config
# reference documents none, and automatic admin creation is still an open
# feature request upstream (gotson/komga#104). The supported headless path is
# the claim endpoint, which is exactly what the web setup form posts to:
#
#   GET  /api/v1/claim  → {"isClaimed": bool}
#   POST /api/v1/claim  → headers X-Komga-Email / X-Komga-Password
#
# Idempotent: the account and its keys live in Komga's database under the
# bind-mounted stateDir, not the ephemeral rootfs, so a reboot re-runs this
# harmlessly and the claim step no-ops.
#
# The API key's plaintext is only guaranteed on the create response, so an
# existing key with our comment is deleted and reminted rather than read back.
# That keeps exactly one key alive under this comment instead of accumulating
# one per boot.
#
# Credentials are read from the environment (KOMGA_ADMIN_USER /
# KOMGA_ADMIN_PASSWORD) and reach curl through a config file on stdin, never
# argv — argv is world-readable in /proc.
#
# Args:
#   --base URL        Komga base URL (http://127.0.0.1:25600)
#   --key-file PATH   where to write KOMGA_API_KEY=…
#                     (/run/komga/api-key.env)

base=http://127.0.0.1:25600
key_file=/run/komga/api-key.env
comment=configuratarr

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

# Fail fast on missing credentials. `set -u` would catch an unset variable in the
# printf calls below, but not an empty one — and an empty password would claim the
# server with a blank admin credential. Check both conditions explicitly.
for v in KOMGA_ADMIN_USER KOMGA_ADMIN_PASSWORD; do
  eval "val=\${$v-}"
  if [ -z "$val" ]; then
    echo "komga-setup: $v is unset or empty — refusing to claim the server" \
      "(check the container's envFile bind mount)" >&2
    exit 1
  fi
done

# Basic-auth credentials for curl, off argv.
auth() {
  printf 'user = "%s:%s"\n' "$KOMGA_ADMIN_USER" "$KOMGA_ADMIN_PASSWORD"
}

# The claim headers carry the credentials, so they go through --config too.
claim_headers() {
  printf 'header = "X-Komga-Email: %s"\n' "$KOMGA_ADMIN_USER"
  printf 'header = "X-Komga-Password: %s"\n' "$KOMGA_ADMIN_PASSWORD"
}

until curl -sf "$base/api/v1/claim" >/dev/null; do sleep 1; done

if [ "$(curl -sf "$base/api/v1/claim" | jq -r .isClaimed)" != "true" ]; then
  claim_headers |
    curl -sf -X POST "$base/api/v1/claim" --config - >/dev/null
  echo "komga-setup: claimed server for $KOMGA_ADMIN_USER" >&2
fi

keys_url="$base/api/v2/users/me/api-keys"

# Drop a stale key under our comment so the mint below stays the only one.
old=$(auth | curl -sf --config - "$keys_url" |
  jq -r --arg c "$comment" '.[] | select(.comment == $c) | .id')

for id in $old; do
  auth | curl -sf --config - -X DELETE "$keys_url/$id" >/dev/null
  echo "komga-setup: removed stale key $id" >&2
done

key=$(auth | curl -sf --config - -X POST "$keys_url" \
  -H 'Content-Type: application/json' \
  --data-binary "$(jq -nc --arg c "$comment" '{comment: $c}')" |
  jq -r .key)

if [ -z "$key" ] || [ "$key" = "null" ]; then
  echo "komga-setup: failed to mint an API key" >&2
  exit 1
fi

umask 077
printf 'KOMGA_API_KEY=%s\n' "$key" >"$key_file"
