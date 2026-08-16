# shellcheck shell=bash
# Publish the session state into the WezTerm tab title as "<state>:<epoch>";
# modules/apps/wezterm/config.lua renders it as a coloured pill.
#
# The mux CLI is the only channel available here: hook subprocesses have no
# controlling terminal, so /dev/tty cannot be opened and the OSC 1337 user-var
# route is out. set-tab-title targets the tab owning $WEZTERM_PANE, which is
# inherited from the pane Claude runs in.
#
# Called with the state as $1; no argument clears the badge.
state=${1:-}

# Nothing to label when Claude is not running under WezTerm.
[ -n "${WEZTERM_PANE:-}" ] || exit 0

# Notification fires both for permission prompts and for idle pings. The raw
# payload on stdin tells them apart without needing a JSON parser.
if [ "$state" = "notification" ]; then
  if grep -q permission_prompt; then
    state=attention
  else
    state=idle
  fi
fi

if [ -z "$state" ]; then
  wezterm cli set-tab-title '' || true
else
  wezterm cli set-tab-title "$state:$(date +%s)" || true
fi
