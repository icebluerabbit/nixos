/*
  seasonpackarr — hardlinks already-downloaded episodes into an announced season
  pack so the pack seeds without re-downloading it
  https://github.com/nuxencs/seasonpackarr

  Binary is pkgs.homelab.seasonpackarr (see packages/seasonpackarr); nixpkgs has
  no package and upstream ships no NixOS module, so the unit is declared here.

  Config is generated from the options below into a store directory —
  seasonpackarr appends "config.yaml" to whatever --config is given, so the flag
  takes a directory, which is exactly what writeTextDir produces. JSON is valid
  YAML, so the file can be generated rather than templated.

  The API token is the one value that cannot live in the store. It arrives as
  SEASONPACKARR__API_TOKEN from envFile: New() runs defaults(), then load() for
  the file, then loadFromEnv() (config.go:368-375), so the environment wins.
*/
_: {
  den.aspects.containers._.seasonpackarr = {
    aspect-name = "containers.seasonpackarr";
    includes = [ ];

    provides.to-hosts.nixos =
      {
        config,
        lib,
        pkgs,
        ...
      }:
      {
        options.containerConfigs.seasonpackarr =
          (import ./_options.nix {
            inherit lib;
            defaultPort = 42069;
            # Share qBittorrent's UID/GID: fs.protected_hardlinks only lets a
            # user hardlink files it owns (or has rw on), and every source file
            # here is qBittorrent-owned download data.
            defaultUid = 5019;
            defaultGid = 5019;
          })
          // {
            envFile = lib.mkOption {
              type = lib.types.nullOr lib.types.path;
              default = null;
              description = "Host path to the environment file holding SEASONPACKARR__API_TOKEN";
            };
            preImportPath = lib.mkOption {
              type = lib.types.str;
              description = ''
                Directory the matched episodes are hardlinked into, and the save
                path the season pack is added with. Must be on the same
                filesystem as qBittorrent's download data (hardlinks) and must
                already exist — seasonpackarr exits at startup otherwise.
              '';
            };
            qbittorrentHost = lib.mkOption {
              type = lib.types.str;
              description = "qBittorrent host; a bare address gets http:// prepended by buildHost";
            };
            qbittorrentPort = lib.mkOption {
              type = lib.types.port;
              description = "qBittorrent WebUI port";
            };
            parseTorrentFile = lib.mkOption {
              type = lib.types.bool;
              default = true;
              description = ''
                Take the season pack's folder and file names from the .torrent
                rather than the announce name. Upstream defaults this to false;
                the two names differ often enough (DDPA5.1 vs DDP5.1) that a
                mismatch re-downloads the whole pack instead of hardlinking it.
              '';
            };
            smartMode = lib.mkOption {
              type = lib.types.bool;
              default = true;
              description = "Only grab a pack when enough of its season is already held from the same release";
            };
            smartModeThreshold = lib.mkOption {
              type = lib.types.float;
              default = 0.51;
              description = ''
                Fraction of a season that must already be held for smartMode to
                accept a pack. Above 0.5 this also bounds it to one pack per
                season: distinct episode sets partition the season, so two
                release groups cannot both clear the threshold.
              '';
            };
            logLevel = lib.mkOption {
              type = lib.types.enum [
                "ERROR"
                "WARN"
                "INFO"
                "DEBUG"
                "TRACE"
              ];
              default = "INFO";
              description = "seasonpackarr log level";
            };
            notificationWebhook = lib.mkOption {
              type = lib.types.nullOr lib.types.str;
              default = null;
              description = ''
                URL seasonpackarr POSTs its notifications to, rendered into
                upstream's `notifications.discord`.

                Upstream only ships a Discord notifier, but it validates
                nothing: discord.go hands this string straight to
                http.NewRequest and treats 200 or 204 as delivered. So a plain
                HTTP collector works — the homelab points it at the container's
                own Vector sidecar, which republishes the Discord embed to MQTT
                and derives pack-outcome metrics from its title.
              '';
            };
            notificationLevel = lib.mkOption {
              type = lib.types.listOf (
                lib.types.enum [
                  "MATCH"
                  "INFO"
                  "ERROR"
                ]
              );
              default = [
                "MATCH"
                "ERROR"
              ];
              description = "Which events are reported; INFO adds the per-announce rejection reasons";
            };
            fuzzyMatching = lib.mkOption {
              default = { };
              description = "Which release attributes are compared leniently against the client's episodes";
              type = lib.types.submodule {
                options = {
                  skipRepackCompare = lib.mkOption {
                    type = lib.types.bool;
                    default = false;
                    description = "Treat a REPACK episode as matching a non-repacked pack";
                  };
                  simplifyHdrCompare = lib.mkOption {
                    type = lib.types.bool;
                    default = false;
                    description = "Treat HDR10 / HDR10+ / HDR+ as plain HDR";
                  };
                  simplifyWebCompare = lib.mkOption {
                    type = lib.types.bool;
                    default = false;
                    description = "Treat WEB-DL as WEB";
                  };
                  skipYearCompare = lib.mkOption {
                    type = lib.types.bool;
                    default = false;
                    description = "Treat a release without a year as matching one with a year";
                  };
                };
              };
            };
          };

        config = {
          containers.seasonpackarr =
            let
              cfg = config.containerConfigs.seasonpackarr;
              # Resolved from host pkgs so the homelab overlay is in scope; both
              # store paths pull into the container closure via ExecStart.
              seasonpackarrBin = lib.getExe pkgs.homelab.seasonpackarr;
              configDir = pkgs.writeTextDir "config.yaml" (
                builtins.toJSON {
                  host = "0.0.0.0";
                  inherit (cfg)
                    port
                    logLevel
                    parseTorrentFile
                    smartMode
                    smartModeThreshold
                    fuzzyMatching
                    ;
                  # buildHost prepends http:// to a bare host and joins the port,
                  # so the two stay separate fields rather than one URL.
                  clients.default = {
                    type = "qbittorrent";
                    host = cfg.qbittorrentHost;
                    port = cfg.qbittorrentPort;
                    inherit (cfg) preImportPath;
                  };
                  notifications = {
                    inherit (cfg) notificationLevel;
                  }
                  // lib.optionalAttrs (cfg.notificationWebhook != null) {
                    discord = cfg.notificationWebhook;
                  };
                }
              );
            in
            {
              inherit (cfg) autoStart;
              ephemeral = true;
              privateNetwork = true;
              inherit (cfg) hostBridge;
              localMacAddress = cfg.macAddress;
              inherit (cfg) localAddress;
              bindMounts =
                cfg.bindMounts
                // (lib.optionalAttrs (cfg.envFile != null) {
                  "/run/secrets/seasonpackarr-env" = {
                    hostPath = cfg.envFile;
                    isReadOnly = true;
                  };
                });

              config =
                { pkgs, lib, ... }:
                {
                  imports = [ (import ./_base.nix { inherit lib cfg; }) ];

                  users.users.seasonpackarr = {
                    uid = lib.mkForce cfg.uid;
                    group = "seasonpackarr";
                    isSystemUser = true;
                  };
                  users.groups.seasonpackarr.gid = lib.mkForce cfg.gid;

                  systemd.services.seasonpackarr = {
                    description = "seasonpackarr — season pack hardlinker for autobrr";
                    wantedBy = [ "multi-user.target" ];
                    after = [ "network.target" ];
                    serviceConfig = {
                      # /mnt/media is autofs on the host, and systemd-tmpfiles
                      # skips anything under an autofs mount, so preImportPath
                      # has to be created from inside the unit.
                      ExecStartPre = "${pkgs.coreutils}/bin/mkdir -p ${cfg.preImportPath}";
                      ExecStart = "${seasonpackarrBin} start --config ${configDir}";
                      EnvironmentFile = lib.mkIf (cfg.envFile != null) "/run/secrets/seasonpackarr-env";
                      User = "seasonpackarr";
                      Group = "seasonpackarr";
                      Restart = "on-failure";
                      RestartSec = "5s";
                    };
                  };
                };
            };
        };
      };
  };
}
