# shellcheck shell=bash
# Turn LazyLibrarian's notify hook arguments into JSON and POST them.
#
# LazyLibrarian is the caller and invokes the hook as
#   <script> key1 val1 key2 val2 …
# with the book's whole DB row (BookID, BookName, AuthorName, Status, Source,
# Event, …). The notify_* fan-out that feeds Slack/Apprise only gets a flat
# prose string with no BookID, so this is the only structured hook available.
#
# Because LazyLibrarian controls argv, this script's own options must be passed
# ahead of a `--` separator by whatever wires the hook up.
#
# Args:
#   --db PATH        LazyLibrarian sqlite db, opened read-only, for the
#                    AuthorID -> AuthorName join
#   --endpoint URL   where to POST the JSON
#   --              everything after this is LazyLibrarian's key/value pairs

db=""
endpoint=""

while [ "$#" -gt 0 ]; do
  case "$1" in
  --db)
    db="$2"
    shift 2
    ;;
  --endpoint)
    endpoint="$2"
    shift 2
    ;;
  --)
    shift
    break
    ;;
  *)
    echo "unknown argument: $1" >&2
    exit 2
    ;;
  esac
done

if [ -z "$endpoint" ]; then
  echo "--endpoint is required" >&2
  exit 2
fi

args=()
authorId=""
while [ "$#" -ge 2 ]; do
  if [ "$1" = "AuthorID" ]; then
    authorId="$2"
  fi
  args+=(--arg "$1" "$2")
  shift 2
done

# The payload is a `SELECT * from books` row, which carries AuthorID but not
# AuthorName — that lives in `authors`. Join it here so Matrix/HA get a name.
if [ -n "$authorId" ] && [ -n "$db" ]; then
  authorName=$(sqlite3 "file:$db?mode=ro" \
    "SELECT AuthorName FROM authors WHERE AuthorID='$authorId';" 2>/dev/null || true)
  if [ -n "$authorName" ]; then
    args+=(--arg AuthorName "$authorName")
  fi
fi

jq -n "${args[@]}" '$ARGS.named' |
  curl -sS -X POST -H 'Content-Type: application/json' \
    --data-binary @- "$endpoint"
