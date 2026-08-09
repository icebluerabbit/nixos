#!/usr/bin/env bash
# Register SSH public keys on gitea's `admin` account for git-over-ssh push.
#
# Self-contained: waits for the local gitea API, mints an ephemeral access token
# via the gitea CLI (no external/stored token), then adds any keys not already
# present. Idempotent — a per-key-set marker short-circuits subsequent boots.
#
# Args : each positional arg is one authorized_keys line ("ssh-ed25519 AAAA... comment").
# Env  : GITEA_API  base URL of the local gitea API (e.g. http://localhost:3000)
#        MARKER     path to a sentinel file; skip entirely if it already exists
# PATH : must provide gitea, curl, jq, awk, coreutils (set by the systemd unit).
set -euo pipefail

api="${GITEA_API:-http://localhost:3000}"
marker="${MARKER:-}"

[ -n "$marker" ] && [ -f "$marker" ] && exit 0

# Wait for the API to answer (gitea.service being "started" != HTTP ready).
for _ in $(seq 1 60); do
  curl -sf "$api/api/healthz" >/dev/null 2>&1 && break
  sleep 2
done

# the gitea CLI needs the config file explicitly (GITEA_CUSTOM/WORK_DIR env alone
# don't point `gitea admin` at app.ini — it errors "Unable to load config file").
cfg="${GITEA_CUSTOM:-/var/lib/gitea/custom}/conf/app.ini"
tok="$(gitea --config "$cfg" admin user generate-access-token --username admin \
  --scopes write:user --token-name "keyreg-$(date +%s)" --raw)"
[ -n "$tok" ]

existing="$(curl -sf -H "Authorization: token $tok" "$api/api/v1/user/keys")"

for pub in "$@"; do
  keypart="$(printf '%s' "$pub" | awk '{print $2}')"
  title="$(printf '%s' "$pub" | awk '{print $3}')"
  if ! printf '%s' "$existing" | grep -qF "$keypart"; then
    curl -sf -X POST -H "Authorization: token $tok" -H "Content-Type: application/json" \
      "$api/api/v1/user/keys" \
      -d "$(jq -n --arg t "$title" --arg k "$pub" '{title: $t, key: $k}')"
  fi
done

[ -n "$marker" ] && touch "$marker"
exit 0
