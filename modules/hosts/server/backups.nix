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
                # exclude reconstructible caches/churn: metrics/log/trace stores,
                # esphome toolchains, jellyfin metadata images, *arr MediaCover,
                # and youtarr's hoarded yt-dlp .info.json sidecars (9G of scrape cruft)
                ExecStart = lib.escapeShellArgs (
                  [
                    backup
                    "--tag"
                    "appdata"
                  ]
                  ++
                    lib.concatMap
                      (p: [
                        "--exclude"
                        p
                      ])
                      [
                        "${snap}/loki"
                        "${snap}/tempo"
                        "${snap}/esphome/.platformio"
                        "${snap}/esphome/.cache"
                        "${snap}/jellyfin/metadata"
                        "${snap}/jellyfin/log"
                        "${snap}/youtarr/config/jobs/info"
                        "${snap}/youtarr/.cache"
                        "**/MediaCover"
                      ]
                  ++ mqttArgs
                  ++ [ snap ]
                );
                # always drop the snapshot, even on failure
                ExecStopPost = "-${btrfs} subvolume delete ${snap}";
              };
            };

            # separate unit (via OnFailure) so a crash/nonzero in the main job still reports
            restic-appdata-notify-fail = {
              description = "report failed appdata backup to Matrix";
              serviceConfig = {
                Type = "oneshot";
                # No RESTIC_REPOSITORY in this unit's environment, so name the repo.
                ExecStart = lib.escapeShellArgs (
                  [
                    notifyFail
                    "--unit"
                    "restic-appdata.service"
                    "--repo"
                    repo
                  ]
                  ++ mqttArgs
                );
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
