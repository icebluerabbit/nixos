# shellcheck shell=bash
# Load an EasyEffects preset, retrying until the filter chain actually exists.
#
# `--load-preset` at daemon start races the PipeWire graph: EE issues the load
# before its own easyeffects_sink filter has registered, so the effects chain
# never instantiates and audio passes through unprocessed. `easyeffects -l`
# always exits 0, so the only reliable completion signal is the node showing up
# in the graph.
#
# EasyEffects 8.x (Qt) does single-instance IPC over a QLocalServer socket at
# $XDG_RUNTIME_DIR/EasyEffectsServer, NOT D-Bus. `easyeffects -l` with no live
# server does not fail — it takes the *server* path and becomes a full second
# instance that never exits. So: wait for the socket before calling, and put a
# timeout on every call, because a socket left behind by a SIGKILLed daemon
# looks alive to stat(2) but refuses connections.
#
# Gate on the socket, never on easyeffects.lock — the lock is created on the
# server path *before* listen(), so an aborted run leaves a lock and no socket.
#
# Args:
#   --easyeffects PATH   easyeffects binary (required)
#   --preset NAME        preset to load (required)
#   --node NAME          graph node that proves the chain came up (required)
#   --socket PATH        single-instance socket ($XDG_RUNTIME_DIR/EasyEffectsServer)
#   --socket-wait SECS   how long to wait for that socket (10)
#   --call-timeout SECS  per-invocation limit on `easyeffects -l` (3)
#   --retries N          load/check attempts (12)
#   --interval SECS      delay between attempts (0.5)
#
# Worst case is socket-wait + retries * (call-timeout + interval); keep that
# under the unit's TimeoutStartSec.
#
# Always exits 0: a preset that never loads must not fail the user session.

easyeffects=""
preset=""
node=""
socket="${XDG_RUNTIME_DIR:-/run/user/$(id -u)}/EasyEffectsServer"
socket_wait=10
call_timeout=3
retries=12
interval=0.5

while [ "$#" -gt 0 ]; do
  case "$1" in
  --easyeffects)
    easyeffects="$2"
    shift 2
    ;;
  --preset)
    preset="$2"
    shift 2
    ;;
  --node)
    node="$2"
    shift 2
    ;;
  --socket)
    socket="$2"
    shift 2
    ;;
  --socket-wait)
    socket_wait="$2"
    shift 2
    ;;
  --call-timeout)
    call_timeout="$2"
    shift 2
    ;;
  --retries)
    retries="$2"
    shift 2
    ;;
  --interval)
    interval="$2"
    shift 2
    ;;
  *)
    echo "unknown argument: $1" >&2
    exit 2
    ;;
  esac
done

if [ -z "$easyeffects" ] || [ -z "$preset" ] || [ -z "$node" ]; then
  echo "--easyeffects, --preset and --node are all required" >&2
  exit 2
fi

# Poll rather than inotify: the caller is normally already gated on this path by
# a .path unit, so this only covers the stale-socket and manual-run cases.
waited=0
while [ ! -S "$socket" ]; do
  if [ "$waited" -ge "$((socket_wait * 4))" ]; then
    echo "no server socket at $socket after ${socket_wait}s; giving up" >&2
    exit 0
  fi
  sleep 0.25
  waited=$((waited + 1))
done

for _ in $(seq 1 "$retries"); do
  # timeout, not `|| true`, is what stops a self-promoted second instance
  # from blocking here forever.
  timeout "$call_timeout" "$easyeffects" -l "$preset" || true
  if pw-cli list-objects Node 2>/dev/null | grep -q "$node"; then
    exit 0
  fi
  sleep "$interval"
done

echo "$node never appeared in the graph; preset may not be applied" >&2
exit 0
