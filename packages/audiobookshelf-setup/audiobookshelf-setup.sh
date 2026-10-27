# shellcheck shell=bash
# Initialize a fresh Audiobookshelf server and export an API key for downstream
# consumers (configuratarr).
#
# Audiobookshelf creates its root user through the same endpoint the web setup
# form posts to, so no UI is needed (server/Server.js):
#
#   GET  /status  → {"isInit": bool}   // isInit = Database.hasRootUser
#   POST /init    → {"newRoot": {"username": …, "password": …}}
#
# /init returns 500 once a root user exists, hence the isInit gate.
#
# Idempotent: the root user and its keys live in Audiobookshelf's database under
# the bind-mounted dataDir, not the ephemeral rootfs, so a reboot re-runs this
# harmlessly and the init step no-ops.
#
# The API key's plaintext is only returned on create, so an existing key with
# our name is deleted and reminted rather than read back. That keeps exactly one
# key alive under this name instead of accumulating one per boot.
#
# Credentials are read from the environment (AUDIOBOOKSHELF_ADMIN_USER /
# AUDIOBOOKSHELF_ADMIN_PASSWORD) and reach curl through stdin or a config file,
# never argv — argv is world-readable in /proc.
#
# Args:
#   --base URL        Audiobookshelf base URL (http://127.0.0.1:13378)
#   --key-file PATH   where to write AUDIOBOOKSHELF_API_KEY=…
#                     (/run/audiobookshelf/api-key.env)

base=http://127.0.0.1:13378
key_file=/run/audiobookshelf/api-key.env
name=configuratarr

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

# Fail fast on missing credentials. `set -u` does NOT catch this: the values are
# read by jq via env.X, which yields null rather than erroring, and POST /init
# happily accepts {"username":null,"password":null} — audiobookshelf then falls
# back to username "root" with an EMPTY password and logs only a warning. That
# is a passwordless admin account reachable from the VLAN, so refuse to run.
for v in AUDIOBOOKSHELF_ADMIN_USER AUDIOBOOKSHELF_ADMIN_PASSWORD; do
  eval "val=\${$v-}"
  if [ -z "$val" ]; then
    echo "audiobookshelf-setup: $v is unset or empty — refusing to" \
      "initialize (check the container's envFile bind mount)" >&2
    exit 1
  fi
done

creds() {
  jq -nc '{
    username: env.AUDIOBOOKSHELF_ADMIN_USER,
    password: env.AUDIOBOOKSHELF_ADMIN_PASSWORD
  }'
}

# Bearer header for curl, off argv.
bearer() {
  printf 'header = "Authorization: Bearer %s"\n' "$1"
}

until curl -sf "$base/status" >/dev/null; do sleep 1; done

if [ "$(curl -sf "$base/status" | jq -r .isInit)" != "true" ]; then
  creds | jq -c '{newRoot: .}' |
    curl -sf -X POST "$base/init" \
      -H 'Content-Type: application/json' --data-binary @- >/dev/null
  echo "audiobookshelf-setup: created root user" >&2
fi

# One login, two values out of it: the session token, and the user id that
# ApiKeyController.create requires in the body (it 400s without a valid userId).
login=$(creds |
  curl -sf -X POST "$base/login" \
    -H 'Content-Type: application/json' --data-binary @-)

# 2.26 moved the session token to user.accessToken; older builds use user.token.
token=$(printf '%s' "$login" | jq -r '.user.accessToken // .user.token')
user_id=$(printf '%s' "$login" | jq -r '.user.id')

if [ -z "$token" ] || [ "$token" = "null" ]; then
  echo "audiobookshelf-setup: login did not return a token" >&2
  exit 1
fi

if [ -z "$user_id" ] || [ "$user_id" = "null" ]; then
  echo "audiobookshelf-setup: login did not return a user id" >&2
  exit 1
fi

keys_url="$base/api/api-keys"

# Drop a stale key under our name so the mint below stays the only one.
old=$(bearer "$token" | curl -sf --config - "$keys_url" |
  jq -r --arg n "$name" '.apiKeys[]? | select(.name == $n) | .id')

for id in $old; do
  bearer "$token" |
    curl -sf --config - -X DELETE "$keys_url/$id" >/dev/null
  echo "audiobookshelf-setup: removed stale key $id" >&2
done

# userId is mandatory (400 without it) and isActive is stored as !!isActive, so
# both must be present. The plaintext key comes back only on this create call,
# nested as .apiKey.apiKey alongside the record's own fields.
key=$(bearer "$token" | curl -sf --config - -X POST "$keys_url" \
  -H 'Content-Type: application/json' \
  --data-binary "$(jq -nc --arg n "$name" --arg u "$user_id" \
    '{name: $n, userId: $u, isActive: true}')" |
  jq -r '.apiKey.apiKey // .apiKey')

if [ -z "$key" ] || [ "$key" = "null" ]; then
  echo "audiobookshelf-setup: failed to mint an API key" >&2
  exit 1
fi

umask 077
printf 'AUDIOBOOKSHELF_API_KEY=%s\n' "$key" >"$key_file"
