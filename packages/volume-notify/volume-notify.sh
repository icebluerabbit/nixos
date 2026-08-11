# shellcheck shell=bash
# Show a desktop notification whenever the default sink's volume or mute state
# changes.
#
# PipeWire has no "volume changed" signal to wait on, so this subscribes to
# pactl's event stream, filters sink events, and re-reads the volume — pactl
# reports that something on a sink changed, not what, so the previous reading is
# kept and compared to suppress duplicate notifications.
#
# The synchronous hint (x-canonical-private-synchronous:volume) makes the
# notification daemon replace the previous popup instead of stacking them.
#
# Args:
#   --sink SPEC       sink to watch (@DEFAULT_AUDIO_SINK@)
#   --app-name NAME   notification app name (System)
#   --timeout MS      notification timeout (2000)

sink='@DEFAULT_AUDIO_SINK@'
app_name=System
timeout_ms=2000

while [ "$#" -gt 0 ]; do
  case "$1" in
  --sink)
    sink="$2"
    shift 2
    ;;
  --app-name)
    app_name="$2"
    shift 2
    ;;
  --timeout)
    timeout_ms="$2"
    shift 2
    ;;
  *)
    echo "unknown argument: $1" >&2
    exit 2
    ;;
  esac
done

# Tolerated rather than fatal: with no sink present (every card disabled, or the
# headset simply powered off) wpctl exits non-zero, and the service should sit
# waiting for one to appear rather than crash-looping on Restart=always.
read_volume() {
  wpctl get-volume "$sink" 2>/dev/null || true
}

last_state=$(read_volume)

pactl subscribe | while read -r event; do
  case "$event" in
  *"on sink"*) ;;
  *) continue ;;
  esac

  current_state=$(read_volume)
  [ -n "$current_state" ] || continue
  [ "$current_state" != "$last_state" ] || continue

  # "Volume: 0.50" or "Volume: 0.50 [MUTED]"
  volume=$(printf '%s\n' "$current_state" | awk '{print int($2 * 100)}')

  case "$current_state" in
  *"[MUTED]"*)
    notify-send -a "$app_name" \
      -h string:x-canonical-private-synchronous:volume \
      -i audio-volume-muted \
      -t "$timeout_ms" \
      "Volume: Muted"
    ;;
  *)
    icon=audio-volume-low
    if [ "$volume" -gt 66 ]; then
      icon=audio-volume-high
    elif [ "$volume" -gt 33 ]; then
      icon=audio-volume-medium
    fi
    notify-send -a "$app_name" \
      -h string:x-canonical-private-synchronous:volume \
      -h int:value:"$volume" \
      -i "$icon" \
      -t "$timeout_ms" \
      "Volume: $volume%"
    ;;
  esac

  last_state="$current_state"
done
