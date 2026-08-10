# shellcheck shell=bash
tmp=$(mktemp -d /tmp/rip.XXXXXXXX)
echo ">> ripping to $tmp ..."
whipper cd rip -O "$tmp" || true

mapfile -t logs < <(find "$tmp" -type f -name '*.log')
if [ "${#logs[@]}" -eq 0 ]; then
  echo "!! no .log in $tmp — rip produced nothing. discarding." >&2
  rm -rf "$tmp"
  exit 1
fi

albumdir=$(dirname "${logs[0]}")
flacs=$(find "$tmp" -type f -name '*.flac' | wc -l)
ripped=0
notver=0
for f in "${logs[@]}"; do
  ripped=$((ripped + $(grep -ci 'accurately ripped' "$f" || true)))
  notver=$((notver + $(grep -ci 'cannot be verified' "$f" || true)))
done

{
  echo "==== AccurateRip summary: $tmp ===="
  echo "FLAC tracks : $flacs"
  echo "AccurateRip : $ripped accurate, $notver not verified"
  echo
  echo "---- per-track ----"
  grep -hiE 'track [0-9]|accurately ripped|cannot be verified|status:|peak level' \
    "${logs[@]}" || true
  echo "===================================="
} | tee "$albumdir/whippit-summary.log"
echo

if [ "$notver" -gt 0 ]; then
  echo "!! $notver track(s) NOT AccurateRip-verified — rare disc or bad rip." >&2
fi

read -rp "Keep this rip for beets? [y/N] " ans
case "$ans" in
y | Y)
  mkdir -p /mnt/media/rips
  # cp -a (not mv): /tmp is tmpfs and /mnt/media is NFS, so this is
  # always a cross-device copy — and mv cannot merge into an existing
  # non-empty dir (whipper's Type dir, e.g. ep/, may already be there
  # from a prior rip), failing with "Directory not empty". cp -a merges
  # dir-by-dir; then drop the tmp staging.
  cp -a "$tmp"/. /mnt/media/rips/
  rm -rf "$tmp"
  echo ">> promoted album subdirs to /mnt/media/rips/ — beets will import"
  ;;
*)
  rm -rf "$tmp"
  echo ">> discarded $tmp"
  ;;
esac
