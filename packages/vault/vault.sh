# shellcheck shell=bash
# Open, close or inspect the on-demand LUKS vault, and run the Vault media
# tagger (https://github.com/aericocode/Vault) against it.
#
# Deliberately not a systemd unit pair. Unlocking this volume is an interactive,
# physically-present act, and `cryptsetup open` talks to the terminal directly
# rather than routing a prompt through systemd-ask-password — which on a desktop
# session means hunting for a password agent that may not be running.
#
# Mount options are NOT repeated here. The mountpoint is declared in
# `fileSystems` with `noauto`, so `mount <mountpoint>` reads them out of fstab
# and the aspect stays the single source of truth.
#
# Args:
#   open | close | status | app   (required, positional)
#   --device PATH           backing block device holding the LUKS2 container
#   --name NAME             /dev/mapper name to open it as
#   --mountpoint PATH       where it lands once unlocked
#   --app PATH              pkgs.homelab.vault-app store path (app action)
#   --app-root PATH         writable app root (default: <mountpoint>/app)
#   -- ARGS...              (app only) remaining args passed to the app

action=""
device=""
name=""
mountpoint=""
app=""
app_root=""
app_args=()

while [ "$#" -gt 0 ]; do
  case "$1" in
  open | close | status | app)
    action="$1"
    shift
    ;;
  --device)
    device="$2"
    shift 2
    ;;
  --name)
    name="$2"
    shift 2
    ;;
  --mountpoint)
    mountpoint="$2"
    shift 2
    ;;
  --app)
    app="$2"
    shift 2
    ;;
  --app-root)
    app_root="$2"
    shift 2
    ;;
  --)
    shift
    app_args=("$@")
    break
    ;;
  *)
    echo "unknown argument: $1" >&2
    exit 2
    ;;
  esac
done

if [ -z "$action" ]; then
  echo "usage: vault open|close|status|app" >&2
  exit 2
fi
if [ -z "$device" ] || [ -z "$name" ] || [ -z "$mountpoint" ]; then
  echo "--device, --name and --mountpoint are required" >&2
  exit 2
fi

if [ "$action" = status ]; then
  if [ -e "/dev/mapper/$name" ]; then
    echo "unlocked: /dev/mapper/$name"
  else
    echo "locked: $device"
  fi
  findmnt --noheadings "$mountpoint" || echo "not mounted: $mountpoint"
  exit 0
fi

if [ "$action" = app ]; then
  # The viewer is an ordinary session process: it writes the library, opens a
  # browser and talks to the LM Studio server running as the user. As root it
  # would leave root-owned files inside the vault and break the next
  # unprivileged start, so refuse rather than silently do that.
  if [ "$(id -u)" -eq 0 ]; then
    echo "vault app must NOT run as root — drop sudo and run it as yourself" >&2
    exit 1
  fi
  if [ -z "$app" ]; then
    echo "--app is required for the app action" >&2
    exit 2
  fi
  if ! findmnt -rn "$mountpoint" >/dev/null; then
    echo "vault is locked — run: sudo vault open" >&2
    exit 1
  fi

  app_root="${app_root:-$mountpoint/app}"
  app_store="$app/lib/node_modules/vault"

  # Vault resolves an "app root" and both reads assets from it (css/, images/,
  # player-lib/, db-viewer.html) and writes its own settings into it
  # (vault-settings.json). That cannot be the store path, so packages/vault-app
  # patches the root to follow VAULT_APP_ROOT and it lands here instead: a
  # directory of symlinks to the store tree, with the writable settings files as
  # real files beside them. Mirroring every entry rather than a curated list
  # means an asset upstream adds later is covered without a script change.
  #
  # Nothing is `require`d through these links — Node resolves modules from the
  # store path in the shebang — so they only ever serve as the asset root.
  mkdir -p "$app_root"
  find "$app_root" -maxdepth 1 -type l -delete
  ln -sfn "$app_store"/* "$app_root/"

  # Every path Vault writes to is pinned inside the vault. Left alone they all
  # default to the app root, which would bury the library database and the
  # derived assets in among the symlinked store tree.
  #
  # The `:=` form means an already-exported value wins, matching Vault's own
  # precedence (shell env > .env > default): any knob not named here can still be
  # set in the calling shell, and every VAULT_* name below also has a legacy
  # VIDEO_TAGGER_* alias that is deliberately not used.
  : "${VAULT_APP_ROOT:=$app_root}"
  : "${VAULT_DB:=$mountpoint/db/vault.db}"
  # Encrypted store for derived artifacts (thumbnails, scrub strips, subtitles)
  # while the library is locked; defaults to sitting beside VAULT_DB.
  : "${VAULT_SECURE_ASSETS:=$mountpoint/db/secure_assets.db}"
  : "${VAULT_TEMP:=$mountpoint/cache/temp_frames}"
  : "${VAULT_TEMP_AUDIO:=$mountpoint/cache/temp_audio}"
  : "${VAULT_THUMBS:=$mountpoint/cache/thumbnails}"
  : "${VAULT_SUBS:=$mountpoint/cache/subtitles}"
  : "${VAULT_TRASH:=$mountpoint/media/trash}"
  : "${VAULT_OUTPUT:=$mountpoint/media/sorted_media}"
  : "${VAULT_EXPORTS:=$mountpoint/media/exports}"
  : "${VAULT_DEBUG_LOG:=$mountpoint/logs/bad_json_responses.log}"
  # Whisper/OPUS-MT/diarization model caches — multi-gigabyte downloads, and the
  # only two path variables upstream never renamed to the VAULT_ prefix.
  : "${OPUS_MODEL_DIR:=$mountpoint/models/opus-mt}"
  : "${DIARIZE_MODEL_DIR:=$mountpoint/models/diarize}"

  # AI backend: the LM Studio server on this host (apps.lm-studio) on its default
  # port — start it from LM Studio's Developer tab, or point this at any other
  # OpenAI-compatible endpoint. Comma-separate to load-balance across GPUs.
  : "${LM_STUDIO_URLS:=http://localhost:1234/v1/chat/completions}"
  # With only one model loaded LM Studio ignores this and serves that one. With
  # the embedding model loaded alongside the vision model — the normal case here
  # — it needs telling which one scans use, and an unset name means Vault stops
  # the scan for a picker every run. Upstream's pick for a 24 GB card (SETUP.md
  # §2); the uncensored fine-tune is deliberate, a stock instruct model refuses
  # on some media and the refusal lands as unparseable metadata.
  : "${AI_MODEL:=qwen3.5-9b-uncensored-hauhaucs-aggressive}"
  : "${EMBEDDING_MODEL:=text-embedding-nomic-embed-text-v1.5}"
  : "${MEDIA_TAGGER_PORT:=8765}"
  : "${VAULT_AUTOLOCK_MINUTES:=30}"
  # 1 = no non-loopback traffic at all. Off because the whisper and OPUS-MT
  # models need a one-time fetch; worth flipping once they are on disk.
  : "${VAULT_OFFLINE:=0}"
  # Transcription runs in Python sidecars Vault spawns by this path; they need
  # faster-whisper installed. Subtitles stay unavailable until that exists —
  # everything else works regardless.
  : "${PYTHON_PATH:=python3}"
  : "${WHISPER_DEVICE:=cuda}"

  export VAULT_APP_ROOT VAULT_DB VAULT_SECURE_ASSETS VAULT_TEMP VAULT_TEMP_AUDIO \
    VAULT_THUMBS VAULT_SUBS VAULT_TRASH VAULT_OUTPUT VAULT_EXPORTS \
    VAULT_DEBUG_LOG OPUS_MODEL_DIR DIARIZE_MODEL_DIR LM_STUDIO_URLS AI_MODEL \
    EMBEDDING_MODEL MEDIA_TAGGER_PORT VAULT_AUTOLOCK_MINUTES VAULT_OFFLINE \
    PYTHON_PATH WHISPER_DEVICE

  # VAULT_DB_PASSWORD is deliberately absent. It is the AES key for the whole
  # metadata database; Vault reads it from the real environment only (a value
  # from a .env file is dropped with a warning), so unattended unlock is
  # `VAULT_DB_PASSWORD=$(...) vault app` with the passphrase from an agent —
  # never a literal in this script or in the vault it protects.

  mkdir -p \
    "$(dirname "$VAULT_DB")" \
    "$VAULT_TEMP" "$VAULT_TEMP_AUDIO" "$VAULT_THUMBS" "$VAULT_SUBS" \
    "$VAULT_TRASH" "$VAULT_OUTPUT" "$VAULT_EXPORTS" \
    "$(dirname "$VAULT_DEBUG_LOG")" \
    "$OPUS_MODEL_DIR" "$DIARIZE_MODEL_DIR"

  # No arguments = serve: viewer on 127.0.0.1:$MEDIA_TAGGER_PORT, browser opened,
  # server in this terminal. Anything after `--` goes through instead, so
  # `vault app -- scan /vault/media --recursive` reaches the same CLI.
  exec "$app/bin/vault" "${app_args[@]:-serve}"
fi

if [ "$(id -u)" -ne 0 ]; then
  echo "vault $action must run as root" >&2
  exit 1
fi

case "$action" in
open)
  if [ ! -e "/dev/mapper/$name" ]; then
    # Same perf flags the root volume opens with (see hosts/desktop/disks.nix).
    # --allow-discards lets TRIM reach the SSD, at the cost of leaking which
    # blocks are in use to anyone who can read the raw device; on a drive that
    # spends most of its life locked and powered down that trade is worth the
    # write amplification it avoids.
    cryptsetup open \
      --allow-discards \
      --perf-no_read_workqueue \
      --perf-no_write_workqueue \
      "$device" "$name"
  fi
  findmnt -rn "$mountpoint" >/dev/null || mount "$mountpoint"
  echo "vault open at $mountpoint"
  ;;
close)
  if findmnt -rn "$mountpoint" >/dev/null; then
    umount "$mountpoint"
  fi
  if [ -e "/dev/mapper/$name" ]; then
    cryptsetup close "$name"
  fi
  echo "vault closed"
  ;;
esac
