# shellcheck shell=bash
input=$(cat)
title=$(echo "$input" | jq -r '.title // .notification_type // "Claude Code"')
msg=$(echo "$input" | jq -r '.message // ""')
type=$(echo "$input" | jq -r '.notification_type // ""')

case "$type" in
permission_prompt) icon="dialog-password" ;;
error) icon="dialog-error" ;;
idle) icon="dialog-information" ;;
*) icon="dialog-information" ;;
esac

notify-send -i "$icon" "$title" "$msg"
