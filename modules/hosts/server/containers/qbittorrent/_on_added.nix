# qbt [AutoRun] on-added hook, two jobs:
#  1. Notify Home Assistant, which posts to #qbittorrent.
#  2. Record the event with managarr. `|| true` so a managarr outage can never
#     block the notify — qbt runs these serially and a hang would stall the
#     AutoRun slot.
# Args: $1 name, $2 category, $3 size-bytes, $4 infohash-v1.
#
# qBittorrent has no HTTP webhook; this external-program hook is the only event
# it emits. The infohash is what makes the event joinable — it is the same
# value an *arr reports as downloadId, so "entered the client" and "was grabbed
# by Sonarr" describe one object. HA's payload deliberately omits it: its
# message templates render the other three and adding a field would change what
# they see.
{
  pkgs,
  lib,
  webhook,
  managarr,
}:
pkgs.writeShellScript "qbt-on-added" ''
  ${lib.getExe pkgs.curl} -sS -m 15 -X POST "${webhook}" \
    -H "Content-Type: application/json" \
    --data "$(${lib.getExe pkgs.jq} -nc \
      --arg event added --arg name "$1" --arg category "$2" --arg size "$3" \
      '{event:$event,name:$name,category:$category,size:$size}')"

  ${lib.getExe pkgs.curl} -sS -m 15 -X POST "${managarr}" \
    -H "Content-Type: application/json" \
    --data "$(${lib.getExe pkgs.jq} -nc \
      --arg event added --arg name "$1" --arg category "$2" --arg size "$3" \
      --arg infohash "$4" \
      '{event:$event,name:$name,category:$category,size:$size,infohash:$infohash}')" || true
''
