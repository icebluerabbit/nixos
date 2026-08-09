# backups — nightly restic backup of /var/lib/appdata to the NAS, from a
# transient read-only btrfs snapshot so all containers are captured at one
# consistent point-in-time (crash-consistent; DBs recover on restore). Results
# are published to MQTT (homelab/notify/backups); HA renders them to #backups.
{ den, ... }:
{
  den.aspects.hosts._.server._.backups = {
    aspect-name = "hosts.server.backups";
    includes = [ den.aspects.infra._.clients._.container-mosquitto ];
    nixos =
      {
        config,
        lib,
        pkgs,
        ...
      }:
      let
        source = "/var/lib/appdata";
        snap = "/var/lib/appdata-snapshot";
        repo = "/mnt/data/backups/appdata";
        btrfs = lib.getExe pkgs.btrfs-progs;
        restic = lib.getExe pkgs.restic;
        jq = lib.getExe pkgs.jq;
        mq = config.infra.clients.container-mosquitto;
        pub = "${pkgs.mosquitto}/bin/mosquitto_pub -h ${mq.ip} -p ${toString mq.ports.default} -q 1 -t homelab/notify/backups -m";
      in
      {
        sops.secrets."restic-appdata-password" = { };

        systemd = {
          services = {
            restic-appdata = {
              description = "restic backup of ${source} to NAS";
              # /mnt/data is NFS automount + noauto; force it up (and the source) first
              unitConfig = {
                RequiresMountsFor = [
                  "/mnt/data"
                  source
                ];
                OnFailure = [ "restic-appdata-notify-fail.service" ];
              };
              serviceConfig = {
                Type = "oneshot";
                LoadCredential = "pw:${config.sops.secrets."restic-appdata-password".path}";
                # systemd creates/owns /var/cache/restic; without a cache dir restic aborts
                # ("neither $XDG_CACHE_HOME nor $HOME are defined") since the unit has no $HOME
                CacheDirectory = "restic";
                Environment = [
                  "RESTIC_REPOSITORY=${repo}"
                  "RESTIC_PASSWORD_FILE=%d/pw"
                  "RESTIC_CACHE_DIR=%C/restic"
                ];
                # clean any stale snapshot from a crashed run, then take a fresh ro one
                ExecStartPre = pkgs.writeShellScript "appdata-snap" ''
                  ${btrfs} subvolume delete ${snap} 2>/dev/null || true
                  ${btrfs} subvolume snapshot -r ${source} ${snap}
                '';
                ExecStart = pkgs.writeShellScript "appdata-backup" ''
                  set -euo pipefail
                  ${restic} cat config >/dev/null 2>&1 || ${restic} init
                  # exclude reconstructible caches/churn: metrics/log/trace stores,
                  # esphome toolchains, jellyfin metadata images, *arr MediaCover,
                  # and youtarr's hoarded yt-dlp .info.json sidecars (9G of scrape cruft)
                  summary=$(${restic} backup --tag appdata --json \
                    --exclude=${snap}/loki \
                    --exclude=${snap}/tempo \
                    --exclude=${snap}/esphome/.platformio \
                    --exclude=${snap}/esphome/.cache \
                    --exclude=${snap}/jellyfin/metadata \
                    --exclude=${snap}/jellyfin/log \
                    --exclude=${snap}/youtarr/config/jobs/info \
                    --exclude=${snap}/youtarr/.cache \
                    --exclude='**/MediaCover' \
                    ${snap} | tail -n1)
                  ${restic} forget --tag appdata \
                    --keep-daily 7 --keep-weekly 4 --keep-monthly 6 --prune
                  payload=$(${jq} -nc \
                    --arg host "$(uname -n)" --arg repo "${repo}" --argjson s "$summary" '{
                      event:"backup", host:$host, result:"success", repo:$repo,
                      snapshot:($s.snapshot_id[0:8]),
                      files_new:$s.files_new, files_changed:$s.files_changed,
                      data_added:$s.data_added, duration:($s.total_duration|floor)
                    }')
                  ${pub} "$payload"
                '';
                # always drop the snapshot, even on failure
                ExecStopPost = "-${btrfs} subvolume delete ${snap}";
              };
            };

            # separate unit (via OnFailure) so a crash/nonzero in the main job still reports
            restic-appdata-notify-fail = {
              description = "report failed appdata backup to Matrix";
              serviceConfig = {
                Type = "oneshot";
                ExecStart = pkgs.writeShellScript "appdata-backup-fail" ''
                  err=$(${pkgs.systemd}/bin/journalctl -u restic-appdata.service -o cat -n 3 \
                        | tail -n1 || true)
                  payload=$(${jq} -nc \
                    --arg host "$(uname -n)" --arg repo "${repo}" --arg err "$err" \
                    '{event:"backup",host:$host,result:"failed",repo:$repo,error:$err}')
                  ${pub} "$payload"
                '';
              };
            };
          };

          timers.restic-appdata = {
            description = "nightly restic backup of appdata";
            wantedBy = [ "timers.target" ];
            timerConfig = {
              OnCalendar = "03:00";
              Persistent = true;
            };
          };
        };
      };
  };
}
