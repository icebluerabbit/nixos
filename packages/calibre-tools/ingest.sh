# shellcheck shell=bash
# Convert and import every book dropped in the ingest folder, then remove the
# source. Non-epub input is converted to epub first; if conversion fails the
# original is added as-is.
#
# Runs with nounset only, NOT errexit: one unconvertible or unreadable file must
# not abort the rest of the batch. Every failure path below is handled inline.
#
# calibredb/ebook-convert come from PATH — see library-init.sh for why.
#
# Args: --library DIR --ingest DIR

library=""
ingest=""

while [ "$#" -gt 0 ]; do
  case "$1" in
  --library)
    library="$2"
    shift 2
    ;;
  --ingest)
    ingest="$2"
    shift 2
    ;;
  *)
    echo "unknown argument: $1" >&2
    exit 2
    ;;
  esac
done

if [ -z "$library" ] || [ -z "$ingest" ]; then
  echo "--library and --ingest are both required" >&2
  exit 2
fi

shopt -s nullglob nocaseglob 2>/dev/null || true

find "$ingest" -type f ! -name '.*' | while read -r f; do
  # skip files still being written (size changing)
  s1=$(stat -c %s "$f" 2>/dev/null || echo x)
  sleep 2
  s2=$(stat -c %s "$f" 2>/dev/null || echo y)
  [ "$s1" = "$s2" ] || continue

  ext=${f##*.}
  ext=${ext,,}
  add="$f"
  tmp=""
  if [ "$ext" != "epub" ]; then
    tmp="$(mktemp --suffix=.epub)"
    if ebook-convert "$f" "$tmp" >/dev/null 2>&1; then
      add="$tmp"
    else
      add="$f"
      rm -f "$tmp"
      tmp=""
    fi
  fi
  if calibredb --with-library="$library" add "$add" >/dev/null 2>&1; then
    rm -f "$f"
  fi
  [ -n "$tmp" ] && rm -f "$tmp" || true
done
