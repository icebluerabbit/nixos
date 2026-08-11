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
        mq = config.infra.clients.container-mosquitto;
        # restic-backup is multi-binary, so no mainProgram to getExe.
        backup = "${pkgs.homelab.restic-backup}/bin/restic-backup";
        notifyFail = "${pkgs.homelab.restic-backup}/bin/restic-backup-notify-fail";
        mqttArgs = [
          "--mqtt-host"
          mq.ip
          "--mqtt-port"
          (toString mq.ports.default)
          "--topic"
          "homelab/notify/backups"
        ];
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
                # exclude reconstructible build output — rust target/ dirs alone
                # accounted for 214G of the 217G this disk held before the move
                ExecStart = lib.escapeShellArgs (
                  [
                    backup
                    "--tag"
                    "projects"
                  ]
                  ++
                    lib.concatMap
                      (p: [
                        "--exclude"
                        p
                      ])
                      [
                        "**/target"
                        "**/node_modules"
                        "**/.direnv"
                        "**/.venv"
                        "**/__pycache__"
                        "**/.cache"
                        "**/result"
                        "**/result-*"
                        "*.iso"
                      ]
                  ++ mqttArgs
                  ++ [ source ]
                );
              };
            };

            # separate unit (via OnFailure) so a crash/nonzero in the main job still reports
            restic-projects-notify-fail = {
              description = "report failed projects backup to Matrix";
              serviceConfig = {
                Type = "oneshot";
                # No RESTIC_REPOSITORY in this unit's environment, so name the repo.
                ExecStart = lib.escapeShellArgs (
                  [
                    notifyFail
                    "--unit"
                    "restic-projects.service"
                    "--repo"
                    repo
                  ]
                  ++ mqttArgs
                );
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
