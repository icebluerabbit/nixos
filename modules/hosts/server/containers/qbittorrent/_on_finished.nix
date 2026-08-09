# qbt [AutoRun] on-finished hook, two jobs:
#  1. Ping cross-seed's /api/webhook with the v1 infohash so it searches every
#     indexer for cross-seeds of what just completed and injects matches
#     (ignoreCrossSeeds default true → injected cross-seeds don't loop back).
#     Reads the apikey from a bind-mounted sops secret. `|| true` so a cross-seed
#     failure can't block the notify.
#  2. Notify Home Assistant, which posts to #qbittorrent.
# Args: $1 name, $2 category, $3 size-bytes, $4 infohash-v1.
{
  pkgs,
  lib,
  webhook,
  crossSeed,
}:
pkgs.writeShellScript "qbt-on-finished" ''
  key=$(cat /run/secrets/cross-seed-apikey)
  ${lib.getExe pkgs.curl} -sS -m 30 -X POST \
    "http://${crossSeed.ip}:${toString crossSeed.ports.default}/api/webhook" \
    -H "X-Api-Key: $key" --data-urlencode "infoHash=$4" || true
  ${lib.getExe pkgs.curl} -sS -m 15 -X POST "${webhook}" \
    -H "Content-Type: application/json" \
    --data "$(${lib.getExe pkgs.jq} -nc \
      --arg event finished --arg name "$1" --arg category "$2" --arg size "$3" \
      '{event:$event,name:$name,category:$category,size:$size}')"
''
