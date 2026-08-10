# shellcheck shell=bash
input=$(cat)
msg=$(echo "$input" | jq -r '.last_assistant_message // "Task complete"' | cut -c1-300)
notify-send -i emblem-default "Claude done" "$msg"
