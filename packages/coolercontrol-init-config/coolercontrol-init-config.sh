# shellcheck shell=bash
# Install a coolercontrold config file into place before the daemon starts.
#
# coolercontrold rewrites /etc/coolercontrol/config.toml at runtime, so it cannot
# be a store symlink — it has to be copied in, with the directory created first.
# Runs from an ExecStartPre with a "+" prefix (full privileges, before the
# service's own user drop).
#
# Args:
#   --config PATH     source file to install (required)
#   --dest PATH       destination (/etc/coolercontrol/config.toml)
#   --dir-mode MODE   mode for the destination directory (755)
#   --file-mode MODE  mode for the installed file (644)

config=""
dest=/etc/coolercontrol/config.toml
dir_mode=755
file_mode=644

while [ "$#" -gt 0 ]; do
  case "$1" in
  --config)
    config="$2"
    shift 2
    ;;
  --dest)
    dest="$2"
    shift 2
    ;;
  --dir-mode)
    dir_mode="$2"
    shift 2
    ;;
  --file-mode)
    file_mode="$2"
    shift 2
    ;;
  *)
    echo "unknown argument: $1" >&2
    exit 2
    ;;
  esac
done

if [ -z "$config" ]; then
  echo "--config is required" >&2
  exit 2
fi

install -d -m "$dir_mode" "$(dirname "$dest")"
install -m "$file_mode" "$config" "$dest"
