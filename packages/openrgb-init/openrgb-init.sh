# shellcheck shell=bash
# Drive an OpenRGB device to a fixed colour at boot.
#
# openrgb.service is "started" once its SDK socket is up, but I2C/USB device
# enumeration takes several more seconds — every caller of this otherwise ends
# up reimplementing the same retry loop.
#
# Args:
#   --openrgb PATH     openrgb binary (required)
#   --device NAME      select by device name, as printed by --list-devices
#   --by-type TYPE     select by "Type: <TYPE>" instead, driving devices by
#                      index; OpenRGB can't read a model string off some
#                      hardware (Corsair DDR5) so the names are unusable
#   --expect N         with --by-type, require exactly N matching devices
#   --retries N        attempts, one second apart, to find the device (30)
#   --settle SECS      wait after the device appears, before the first set (0)
#   --passes N         repeat the whole set list N times (1)
#   --set ZONE:COLOR   apply COLOR to zone ZONE; empty ZONE means the whole
#                      device. Repeatable, applied in the order given.
#   --set-delay SECS   sleep after each --set (0)
#   --pass-delay SECS  sleep after each pass (0)

openrgb=""
device=""
by_type=""
expect=""
retries=30
settle=0
passes=1
set_delay=0
pass_delay=0
sets=()

while [ "$#" -gt 0 ]; do
  case "$1" in
  --openrgb)
    openrgb="$2"
    shift 2
    ;;
  --device)
    device="$2"
    shift 2
    ;;
  --by-type)
    by_type="$2"
    shift 2
    ;;
  --expect)
    expect="$2"
    shift 2
    ;;
  --retries)
    retries="$2"
    shift 2
    ;;
  --settle)
    settle="$2"
    shift 2
    ;;
  --passes)
    passes="$2"
    shift 2
    ;;
  --set)
    sets+=("$2")
    shift 2
    ;;
  --set-delay)
    set_delay="$2"
    shift 2
    ;;
  --pass-delay)
    pass_delay="$2"
    shift 2
    ;;
  *)
    echo "unknown argument: $1" >&2
    exit 2
    ;;
  esac
done

if [ -z "$openrgb" ]; then
  echo "--openrgb is required" >&2
  exit 2
fi
if [ -z "$device" ] && [ -z "$by_type" ]; then
  echo "one of --device or --by-type is required" >&2
  exit 2
fi
if [ "${#sets[@]}" -eq 0 ]; then
  echo "at least one --set is required" >&2
  exit 2
fi

# Apply every --set to one device selector, honouring --set-delay.
apply_sets() {
  local selector="$1" spec zone color
  for spec in "${sets[@]}"; do
    zone="${spec%%:*}"
    color="${spec#*:}"
    if [ -n "$zone" ]; then
      "$openrgb" --device "$selector" --mode Direct \
        --zone "$zone" --color "$color" || return 1
    else
      "$openrgb" --device "$selector" --mode Direct \
        --color "$color" || return 1
    fi
    [ "$set_delay" = "0" ] || sleep "$set_delay"
  done
}

# Indices of every device whose "Type:" line matches --by-type. --list-devices
# prints "N: <name>" followed by indented fields, so carry the last index seen.
list_by_type() {
  "$openrgb" --list-devices 2>/dev/null | awk -v want="$by_type" '
    /^[0-9]+:/ { idx = $1; sub(":", "", idx) }
    $0 ~ "Type:[[:space:]]+" want { print idx }
  '
}

selectors=()
for _ in $(seq 1 "$retries"); do
  if [ -n "$by_type" ]; then
    mapfile -t found < <(list_by_type)
    if [ -n "$expect" ]; then
      [ "${#found[@]}" -eq "$expect" ] || {
        sleep 1
        continue
      }
    else
      [ "${#found[@]}" -gt 0 ] || {
        sleep 1
        continue
      }
    fi
    selectors=("${found[@]}")
  else
    "$openrgb" --list-devices 2>/dev/null | grep -qF "$device" || {
      sleep 1
      continue
    }
    selectors=("$device")
  fi
  break
done

if [ "${#selectors[@]}" -eq 0 ]; then
  echo "device not found after ${retries}s" >&2
  exit 1
fi

echo "Device detected: ${selectors[*]}"
[ "$settle" = "0" ] || sleep "$settle"

# The apply itself can still fail while enumeration settles, so retry the whole
# pass list rather than assuming a listed device is a writable one.
for _ in $(seq 1 "$retries"); do
  ok=1
  for pass in $(seq 1 "$passes"); do
    echo "Initialization pass $pass..."
    for selector in "${selectors[@]}"; do
      apply_sets "$selector" || {
        ok=0
        break
      }
    done
    [ "$ok" = "1" ] || break
    [ "$pass_delay" = "0" ] || sleep "$pass_delay"
  done
  if [ "$ok" = "1" ]; then
    echo "Initialization complete."
    exit 0
  fi
  sleep 1
done

echo "failed to apply colours after ${retries} attempts" >&2
exit 1
