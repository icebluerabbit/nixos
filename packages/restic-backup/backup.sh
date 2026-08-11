# shellcheck shell=bash
# Run a restic backup, prune old snapshots, and publish a JSON result to MQTT.
#
# The repository, password file and cache dir come from the standard restic
# environment ($RESTIC_REPOSITORY, $RESTIC_PASSWORD_FILE, $RESTIC_CACHE_DIR),
# which the systemd unit already sets — so nothing here needs to know where the
# repo lives beyond echoing it back in the payload.
#
# Args:
#   --tag TAG           restic tag for this job, also its forget selector
#   --repo REPO         repo shown in the payload ($RESTIC_REPOSITORY)
#   --exclude PATTERN   passed through to restic backup; repeatable
#   --keep-daily N      (7)
#   --keep-weekly N     (4)
#   --keep-monthly N    (6)
#   --mqtt-host HOST    broker to publish to; publishing is skipped if unset
#   --mqtt-port PORT    (1883)
#   --topic TOPIC       (homelab/notify/backups)
#   PATH...             positional; what to back up

tag=""
repo="${RESTIC_REPOSITORY:-}"
excludes=()
keep_daily=7
keep_weekly=4
keep_monthly=6
mqtt_host=""
mqtt_port=1883
topic=homelab/notify/backups
paths=()

while [ "$#" -gt 0 ]; do
  case "$1" in
  --tag)
    tag="$2"
    shift 2
    ;;
  --repo)
    repo="$2"
    shift 2
    ;;
  --exclude)
    excludes+=("--exclude=$2")
    shift 2
    ;;
  --keep-daily)
    keep_daily="$2"
    shift 2
    ;;
  --keep-weekly)
    keep_weekly="$2"
    shift 2
    ;;
  --keep-monthly)
    keep_monthly="$2"
    shift 2
    ;;
  --mqtt-host)
    mqtt_host="$2"
    shift 2
    ;;
  --mqtt-port)
    mqtt_port="$2"
    shift 2
    ;;
  --topic)
    topic="$2"
    shift 2
    ;;
  --*)
    echo "unknown argument: $1" >&2
    exit 2
    ;;
  *)
    paths+=("$1")
    shift
    ;;
  esac
done

if [ -z "$tag" ]; then
  echo "--tag is required" >&2
  exit 2
fi
if [ "${#paths[@]}" -eq 0 ]; then
  echo "at least one path is required" >&2
  exit 2
fi

restic cat config >/dev/null 2>&1 || restic init

summary=$(restic backup --tag "$tag" --json "${excludes[@]}" "${paths[@]}" | tail -n1)

restic forget --tag "$tag" \
  --keep-daily "$keep_daily" \
  --keep-weekly "$keep_weekly" \
  --keep-monthly "$keep_monthly" \
  --prune

[ -n "$mqtt_host" ] || exit 0

payload=$(jq -nc \
  --arg host "$(uname -n)" --arg repo "$repo" --argjson s "$summary" '{
    event:"backup", host:$host, result:"success", repo:$repo,
    snapshot:($s.snapshot_id[0:8]),
    files_new:$s.files_new, files_changed:$s.files_changed,
    data_added:$s.data_added, duration:($s.total_duration|floor)
  }')

mosquitto_pub -h "$mqtt_host" -p "$mqtt_port" -q 1 -t "$topic" -m "$payload"
