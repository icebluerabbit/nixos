# shellcheck shell=bash
# Create an empty Calibre library if none exists.
#
# calibredb initialises an empty metadata.db on first access, so a plain `list`
# is enough; restore_database is the fallback for the case where the directory
# exists but the db does not.
#
# calibredb comes from PATH, not from this package's closure — the caller puts
# pkgs.calibre on the unit's `path` so a several-hundred-MB dependency does not
# ride along with the script.
#
# Args: --library DIR

library=""

while [ "$#" -gt 0 ]; do
  case "$1" in
  --library)
    library="$2"
    shift 2
    ;;
  *)
    echo "unknown argument: $1" >&2
    exit 2
    ;;
  esac
done

if [ -z "$library" ]; then
  echo "--library is required" >&2
  exit 2
fi

if [ ! -f "$library/metadata.db" ]; then
  mkdir -p "$library"
  calibredb --with-library="$library" list >/dev/null 2>&1 || true
  if [ ! -f "$library/metadata.db" ]; then
    calibredb --with-library="$library" restore_database --really-do-it >/dev/null 2>&1 || true
  fi
fi
