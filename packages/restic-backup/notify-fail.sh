# shellcheck shell=bash
# Publish a failure result to MQTT for a restic job that exited non-zero.
#
# Runs from the failed unit's OnFailure=, so it is a separate process: a crash
# or non-zero exit in the backup itself still gets reported.
#
# Args:
#   --unit UNIT         unit whose journal tail becomes the error string
#   --repo REPO         repo shown in the payload ($RESTIC_REPOSITORY)
#   --mqtt-host HOST    broker to publish to (required)
#   --mqtt-port PORT    (1883)
#   --topic TOPIC       (homelab/notify/backups)

unit=""
repo="${RESTIC_REPOSITORY:-}"
mqtt_host=""
mqtt_port=1883
topic=homelab/notify/backups

while [ "$#" -gt 0 ]; do
  case "$1" in
  --unit)
    unit="$2"
    shift 2
    ;;
  --repo)
    repo="$2"
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
  *)
    echo "unknown argument: $1" >&2
    exit 2
    ;;
  esac
done

if [ -z "$unit" ] || [ -z "$mqtt_host" ]; then
  echo "--unit and --mqtt-host are required" >&2
  exit 2
fi

err=$(journalctl -u "$unit" -o cat -n 3 | tail -n1 || true)

payload=$(jq -nc \
  --arg host "$(uname -n)" --arg repo "$repo" --arg err "$err" \
  '{event:"backup",host:$host,result:"failed",repo:$repo,error:$err}')

mosquitto_pub -h "$mqtt_host" -p "$mqtt_port" -q 1 -t "$topic" -m "$payload"
