# backups — nightly restic backup of /projects to the NAS. Unlike the server's
# appdata job there is no btrfs snapshot step: this is a dev tree rather than a
# set of live container DBs, so reading files as they change is acceptable and
# restic tolerates the churn. Results are published to MQTT
# (homelab/notify/backups); HA renders them to #backups.
{ den, ... }:
{
  den.aspects.hosts._.desktop._.backups = {
    aspect-name = "hosts.desktop.backups";
    includes = [ den.aspects.infra._.clients._.container-mosquitto ];
    nixos =
      {
        config,
        lib,
        pkgs,
        ...
      }:
      let
        source = "/projects";
        repo = "/mnt/data/backups/projects";
        restic = lib.getExe pkgs.restic;
        jq = lib.getExe pkgs.jq;
        mq = config.infra.clients.container-mosquitto;
        pub = "${pkgs.mosquitto}/bin/mosquitto_pub -h ${mq.ip} -p ${toString mq.ports.default} -q 1 -t homelab/notify/backups -m";
      in
      {
        sops.secrets."restic-projects-password" = { };

        systemd = {
          services = {
            restic-projects = {
              description = "restic backup of ${source} to NAS";
              # /mnt/data is NFS automount + noauto; force it up (and the source) first
              unitConfig = {
                RequiresMountsFor = [
                  "/mnt/data"
                  source
                ];
                OnFailure = [ "restic-projects-notify-fail.service" ];
              };
              serviceConfig = {
                Type = "oneshot";
                LoadCredential = "pw:${config.sops.secrets."restic-projects-password".path}";
                # systemd creates/owns /var/cache/restic; without a cache dir restic aborts
                # ("neither $XDG_CACHE_HOME nor $HOME are defined") since the unit has no $HOME
                CacheDirectory = "restic";
                Environment = [
                  "RESTIC_REPOSITORY=${repo}"
                  "RESTIC_PASSWORD_FILE=%d/pw"
                  "RESTIC_CACHE_DIR=%C/restic"
                ];
                ExecStart = pkgs.writeShellScript "projects-backup" ''
                  set -euo pipefail
                  ${restic} cat config >/dev/null 2>&1 || ${restic} init
                  # exclude reconstructible build output — rust target/ dirs alone
                  # accounted for 214G of the 217G this disk held before the move
                  summary=$(${restic} backup --tag projects --json \
                    --exclude='**/target' \
                    --exclude='**/node_modules' \
                    --exclude='**/.direnv' \
                    --exclude='**/.venv' \
                    --exclude='**/__pycache__' \
                    --exclude='**/.cache' \
                    --exclude='**/result' \
                    --exclude='**/result-*' \
                    --exclude='*.iso' \
                    ${source} | tail -n1)
                  ${restic} forget --tag projects \
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
              };
            };

            # separate unit (via OnFailure) so a crash/nonzero in the main job still reports
            restic-projects-notify-fail = {
              description = "report failed projects backup to Matrix";
              serviceConfig = {
                Type = "oneshot";
                ExecStart = pkgs.writeShellScript "projects-backup-fail" ''
                  err=$(${pkgs.systemd}/bin/journalctl -u restic-projects.service -o cat -n 3 \
                        | tail -n1 || true)
                  payload=$(${jq} -nc \
                    --arg host "$(uname -n)" --arg repo "${repo}" --arg err "$err" \
                    '{event:"backup",host:$host,result:"failed",repo:$repo,error:$err}')
                  ${pub} "$payload"
                '';
              };
            };
          };

          # 04:30 rather than the server's 03:00 so both hosts aren't hitting the
          # NAS at once; Persistent catches up if the workstation was powered off.
          timers.restic-projects = {
            description = "nightly restic backup of projects";
            wantedBy = [ "timers.target" ];
            timerConfig = {
              OnCalendar = "04:30";
              Persistent = true;
            };
          };
        };
      };
  };
}
