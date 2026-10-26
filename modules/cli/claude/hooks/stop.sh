# shellcheck shell=bash
input=$(cat)
msg=$(echo "$input" | jq -r '.last_assistant_message // "Task complete"' | cut -c1-300)

# notify-send talks to a session dbus, which macOS has no equivalent of —
# there it fails with "Cannot spawn a message bus without a machine-id".
# osascript is the system notifier and needs no package. The message
# arrives as argv, so nothing in it has to be escaped.
if [ "$(uname)" = Darwin ]; then
  /usr/bin/osascript - "$msg" <<'APPLESCRIPT'
on run argv
  display notification (item 1 of argv) with title "Claude done"
end run
APPLESCRIPT
else
  notify-send -i emblem-default "Claude done" "$msg"
fi
