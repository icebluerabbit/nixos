# Hand one finished album to Lidarr for import.
#
# beets calls this once it has written an album into its output directory.
# DownloadedAlbumsScan runs the same code path a completed download takes:
# identify the folder, link it into the artist folder, rename to the configured
# format, and record the extra files alongside it. That is what keeps a CD rip
# and a torrent landing identically, with Lidarr as the only writer into the
# library.
#
# Lidarr identifies the release by parsing the folder NAME, so beets must name
# its output folders "Artist - Album (Year)" the way a download is named. A
# folder called just "We are" fails with "Unknown Artist" before a single tag
# is read.

album=${1:?usage: beets-lidarr-import <album-directory>}

# Lidarr scopes a scan by parsing the folder name for an artist before it reads
# any tags, so it is handed the artist directory, not the album directory. Given
# "…/rips/i‐dle/We are" it parses "We are", finds no such artist and gives up;
# given "…/rips/i‐dle" it resolves the artist and then identifies the album
# underneath from its tags.
target=$(dirname "$album")
: "${LIDARR_URL:?LIDARR_URL is not set}"
: "${LIDARR_API_KEY:?LIDARR_API_KEY is not set}"

# Copy leaves the rip where beets put it and, with Lidarr's copyUsingHardlinks
# enabled on the same filesystem, links rather than duplicates it — so the rip
# survives as its own tree at no extra disk cost. That mirrors how a torrent
# stays in the download directory while Lidarr links it into the library.
mode=${LIDARR_IMPORT_MODE:-Copy}
timeout=${LIDARR_IMPORT_TIMEOUT:-300}
poll=${LIDARR_IMPORT_POLL:-2}
api="${LIDARR_URL%/}/api/v1"

request() {
  curl -fsS -H "X-Api-Key: ${LIDARR_API_KEY}" "$@"
}

payload=$(jq -nc --arg path "$target" --arg mode "$mode" \
  '{name: "DownloadedAlbumsScan", path: $path, importMode: $mode}')

if ! response=$(request -X POST "${api}/command" \
  -H 'Content-Type: application/json' -d "$payload"); then
  echo "beets-lidarr-import: could not reach Lidarr at ${api}" >&2
  exit 1
fi

id=$(jq -r '.id // empty' <<<"$response")
if [[ -z $id ]]; then
  echo "beets-lidarr-import: Lidarr rejected the request for ${album}: ${response}" >&2
  exit 1
fi

echo "beets-lidarr-import: queued command ${id} for ${album}"

# The command is asynchronous, and it completes successfully even when nothing
# matched — that shows up as an "unsuccessful" result rather than a failure.
# Polling for it is the difference between knowing an album was imported and
# assuming it was.
deadline=$((SECONDS + timeout))
while ((SECONDS < deadline)); do
  if ! body=$(request "${api}/command/${id}"); then
    echo "beets-lidarr-import: lost contact with Lidarr while awaiting ${id}" >&2
    exit 1
  fi

  status=$(jq -r '.status // "unknown"' <<<"$body")
  case "$status" in
  completed)
    result=$(jq -r '.result // "unknown"' <<<"$body")
    if [[ $result == successful ]]; then
      echo "beets-lidarr-import: imported ${album}"
      exit 0
    fi
    # Nothing was linked into the library, so the album is visible in the
    # output directory as an album Lidarr could not place.
    echo "beets-lidarr-import: Lidarr matched nothing in ${album}; not linked into the library" >&2
    exit 1
    ;;
  failed | aborted)
    message=$(jq -r '.exception // .message // "no detail given"' <<<"$body")
    echo "beets-lidarr-import: import of ${album} ${status}: ${message}" >&2
    exit 1
    ;;
  esac

  sleep "$poll"
done

echo "beets-lidarr-import: gave up waiting ${timeout}s for command ${id} (${album})" >&2
exit 1
