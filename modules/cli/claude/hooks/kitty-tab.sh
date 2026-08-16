# shellcheck shell=bash
# Tint this session's kitty tab according to its state.
#
# Remote control is the only channel available here: hook subprocesses
# have no controlling terminal, so /dev/tty cannot be opened and the OSC
# escape route is out. KITTY_LISTEN_ON and KITTY_WINDOW_ID are both in the
# hook's environment, inherited from the window Claude runs in.
#
# Only the inactive_* colours are set, so the tint shows purely while the
# tab is unfocused; the tab you are already looking at keeps kitty's own
# colours. Nothing is written to the tab title, which stays free to name
# the task.
#
# States overwrite each other and SessionEnd clears, so nothing here needs
# to know about focus: a tint lasts until the session's next event. The
# one visible consequence is that a finished session stays tinted until
# you give it a new prompt.
#
# Palette comes from stylix via runtimeEnv on the wrapper in claude.nix.
#
# Called with the state as $1; no argument clears the tint.
state=${1:-}

# Nothing to tint when Claude is not running under kitty.
[ -n "${KITTY_WINDOW_ID:-}" ] || exit 0

# Notification fires both for permission prompts and for idle pings. The
# raw payload on stdin tells them apart without needing a JSON parser.
if [ "$state" = "notification" ]; then
  if grep -q permission_prompt; then
    state=attention
  else
    state=idle
  fi
fi

case "$state" in
busy) bg=$CLAUDE_TAB_BUSY ;;
attention) bg=$CLAUDE_TAB_ATTENTION ;;
idle) bg=$CLAUDE_TAB_IDLE ;;
done) bg=$CLAUDE_TAB_DONE ;;
*) bg=none ;;
esac

if [ "$bg" = none ]; then
  fg=none
else
  fg=$CLAUDE_TAB_FG
fi

kitten @ set-tab-color --match "window_id:${KITTY_WINDOW_ID}" \
  "inactive_bg=${bg}" "inactive_fg=${fg}" || true
