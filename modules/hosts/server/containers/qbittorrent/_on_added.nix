# qbt [AutoRun] on-added hook: notify Home Assistant, which posts to #qbittorrent.
# Args (from the AutoRun program string): $1 name, $2 category, $3 size-bytes.
{
  pkgs,
  lib,
  webhook,
}:
pkgs.writeShellScript "qbt-on-added" ''
  ${lib.getExe pkgs.curl} -sS -m 15 -X POST "${webhook}" \
    -H "Content-Type: application/json" \
    --data "$(${lib.getExe pkgs.jq} -nc \
      --arg event added --arg name "$1" --arg category "$2" --arg size "$3" \
      '{event:$event,name:$name,category:$category,size:$size}')"
''
