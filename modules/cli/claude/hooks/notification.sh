# shellcheck shell=bash
input=$(cat)
title=$(echo "$input" | jq -r '.title // .notification_type // "Claude Code"')
msg=$(echo "$input" | jq -r '.message // ""')
type=$(echo "$input" | jq -r '.notification_type // ""')

# See stop.sh: no dbus on macOS, so notify-send cannot run there. Notification
# Center takes no icon, so the freedesktop icon names below are Linux-only.
if [ "$(uname)" = Darwin ]; then
  /usr/bin/osascript - "$msg" "$title" <<'APPLESCRIPT'
on run argv
  display notification (item 1 of argv) with title (item 2 of argv)
end run
APPLESCRIPT
  exit 0
fi

case "$type" in
permission_prompt) icon="dialog-password" ;;
error) icon="dialog-error" ;;
idle) icon="dialog-information" ;;
*) icon="dialog-information" ;;
esac

notify-send -i "$icon" "$title" "$msg"
