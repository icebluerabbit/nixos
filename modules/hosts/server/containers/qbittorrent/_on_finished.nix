# qbt [AutoRun] on-finished hook, three jobs:
#  1. Ping cross-seed's /api/webhook with the v1 infohash so it searches every
#     indexer for cross-seeds of what just completed and injects matches
#     (ignoreCrossSeeds default true → injected cross-seeds don't loop back).
#     Reads the apikey from a bind-mounted sops secret. `|| true` so a cross-seed
#     failure can't block the notify.
#  2. Notify Home Assistant, which posts to #qbittorrent.
#  3. Hand the event to managarr-qbittorrent, the sidecar in this container,
#     which fills in the file list and trackers before relaying it onward.
#     `|| true` for the same reason as above.
# Args: $1 name, $2 category, $3 size-bytes, $4 infohash-v1.
#
# The infohash is the join key: it is what an *arr import webhook reports as
# downloadId, so this event and that one describe the same object. Recording
# both is what makes the torrent→library mapping a stated fact.
{
  pkgs,
  lib,
  webhook,
  crossSeed,
  sidecar,
}:
pkgs.writeShellScript "qbt-on-finished" ''
  key=$(cat /run/secrets/cross-seed-apikey)
  ${lib.getExe pkgs.curl} -sS -m 30 -X POST \
    "http://${crossSeed.ip}:${toString crossSeed.ports.default}/api/webhook" \
    -H "X-Api-Key: $key" --data-urlencode "infoHash=$4" || true

  # HA's payload is left exactly as it was — its message templates render
  # these three fields, and adding one would change what they see.
  ${lib.getExe pkgs.curl} -sS -m 15 -X POST "${webhook}" \
    -H "Content-Type: application/json" \
    --data "$(${lib.getExe pkgs.jq} -nc \
      --arg event finished --arg name "$1" --arg category "$2" --arg size "$3" \
      '{event:$event,name:$name,category:$category,size:$size}')"

  # managarr-qbittorrent, the sidecar running in this container, rather than
  # managarr directly.
  #
  # This is the one place in the stack where a missed event is permanent.
  # qBittorrent keeps no history: once a *arr imports this torrent and removes
  # it, nothing anywhere records that it existed, what files it held or which
  # tracker it came from. The hourly pass is not there to see it.
  #
  # The four macros below are everything qBittorrent will tell a hook. The
  # sidecar answers on the loopback and, while the torrent is still in the
  # client, reads back the file list, the trackers and the real sizes — then
  # relays the lot to managarr in the same flat shape this used to post, so
  # managarr's parser reads what it always read and the rest is additive.
  #
  # `|| true` for the same reason as the calls above: a recorder being down
  # must not fail the hook that notifies.
  ${lib.getExe pkgs.curl} -sS -m 15 -X POST "${sidecar}" \
    -H "Content-Type: application/json" \
    --data "$(${lib.getExe pkgs.jq} -nc \
      --arg event finished --arg name "$1" --arg category "$2" --arg size "$3" \
      --arg infohash "$4" \
      '{event:$event,name:$name,category:$category,size:$size,infohash:$infohash}')" || true
''
