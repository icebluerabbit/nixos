# shellcheck shell=bash
# Keep a set of Kiwix ZIM books in sync with an rsync mirror.
#
# Each book is tracked by its filename prefix; the mirror carries dated files
# (<prefix>_YYYY-MM.zim) so "latest" is the highest version-sorted name. A book
# already present is left alone, otherwise the old copy and any stale .part are
# removed and the new one is fetched to .part before an atomic rename.
#
# Args:
#   --mirror URL             base rsync URL, path up to /zim (required)
#   --zim-dir DIR            where the .zim files live (/zim)
#   --book CATEGORY:PREFIX   repeatable, at least one required

mirror=""
zim_dir=/zim
books=()

while [ "$#" -gt 0 ]; do
  case "$1" in
  --mirror)
    mirror="$2"
    shift 2
    ;;
  --zim-dir)
    zim_dir="$2"
    shift 2
    ;;
  --book)
    books+=("$2")
    shift 2
    ;;
  *)
    echo "unknown argument: $1" >&2
    exit 2
    ;;
  esac
done

if [ -z "$mirror" ]; then
  echo "--mirror is required" >&2
  exit 2
fi
if [ "${#books[@]}" -eq 0 ]; then
  echo "at least one --book CATEGORY:PREFIX is required" >&2
  exit 2
fi

# A book whose glob matches nothing makes rsync exit non-zero, which on its own
# is indistinguishable from the mirror being down. Probe once up front so a real
# outage fails the unit — systemd retries it on RestartSec — instead of being
# reported as "no files" for every book in turn and exiting 0.
if ! rsync --list-only "$mirror/" >/dev/null 2>&1; then
  echo "mirror $mirror is unreachable" >&2
  exit 1
fi

sync_book() {
  local category="$1"
  local prefix="$2"

  echo "[$prefix] Querying mirror for latest..."
  local listing latest
  # Past the probe above, a failed listing means this prefix has no files yet.
  listing=$(rsync --list-only "$mirror/$category/$prefix"'_*.zim' 2>/dev/null) || listing=""
  # grep is guarded: a listing of nothing but .part files would otherwise exit 1
  # and take the whole script down under pipefail.
  latest=$(printf '%s\n' "$listing" |
    awk '{print $NF}' |
    { grep -v '\.part$' || true; } |
    sort -V |
    tail -1)

  if [[ -z $latest ]]; then
    echo "[$prefix] No files found on mirror, skipping"
    return 0
  fi

  if [[ -f "$zim_dir/$latest" ]]; then
    echo "[$prefix] Already have $latest, nothing to do"
    find "$zim_dir" -name "$prefix"'_*.zim.part' -delete
    return 0
  fi

  echo "[$prefix] Syncing $latest..."
  find "$zim_dir" -name "$prefix"'_*.zim' -delete
  find "$zim_dir" -name "$prefix"'_*.zim.part' -not -name "$latest"'.part' -delete

  rsync -av --partial --progress "$mirror/$category/$latest" "$zim_dir/$latest"'.part'
  mv "$zim_dir/$latest"'.part' "$zim_dir/$latest"
  echo "[$prefix] Done: $latest"
}

for book in "${books[@]}"; do
  sync_book "${book%%:*}" "${book#*:}"
done

echo "Kiwix ZIM sync completed."
