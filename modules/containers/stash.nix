# stash — organizer, scraper and player for the adult library
# https://stashapp.cc
#
# The identification and playback half of the adult stack. Jellyfin has no
# metadata provider for this content, and Whisparr — which was meant to acquire
# into /mnt/media/library/xxx — was removed after it proved unable to match
# scene releases. Stash indexes complete/ as well as library/, so tagging no
# longer waits on an acquirer identifying anything first.
#
# SQLite only — the module has no postgres path (settings.database is a file).
# State, generated previews, cache and blobs all live under dataDir, which is
# the bind mount.
_: {
  den.aspects.containers._.stash = {
    aspect-name = "containers.stash";
    includes = [ ];

    provides.to-hosts.nixos =
      {
        config,
        lib,
        pkgs,
        ...
      }:
      let
        hostConfig = config;
        # Resolved from HOST pkgs — the container is built from a separate
        # nixpkgs instance that carries no homelab overlay. These are plain data
        # trees, symlinked into scrapers_path/plugins_path by the module.
        scrapersPkg = pkgs.homelab.stash-community-scrapers;
        pluginsPkg = pkgs.homelab.stash-community-scripts;
        # The Python plugins import `stashapi` (stashapp-tools), which is not in
        # nixpkgs. The module puts a bare python3 on the unit's PATH, so without
        # this they load and immediately die on ImportError.
        pythonEnv = pkgs.python3.withPackages (_: [ pkgs.homelab.stashapp-tools ]);
      in
      {
        options.containerConfigs.stash =
          (import ./_options.nix {
            inherit lib;
            defaultPort = 9999;
            defaultUid = 5042;
            defaultGid = 5042;
          })
          // {
            libraryPaths = lib.mkOption {
              type = lib.types.listOf lib.types.str;
              default = [ "/mnt/media/library/xxx" ];
              description = "Media directories Stash scans for scenes and images";
            };
            parallelTasks = lib.mkOption {
              type = lib.types.ints.positive;
              default = 1;
              description = "Concurrent scan/generate workers; each one runs its own ffmpeg";
            };
            taskInterval = lib.mkOption {
              type = lib.types.nullOr lib.types.str;
              default = null;
              description = "systemd OnCalendar for the scan/identify timer; null disables it";
            };
            jwtSecretKeyFile = lib.mkOption {
              type = lib.types.nullOr lib.types.path;
              default = null;
              description = "Host path to a file holding the JWT signing secret (required by the module)";
            };
            sessionStoreKeyFile = lib.mkOption {
              type = lib.types.nullOr lib.types.path;
              default = null;
              description = "Host path to a file holding the session store secret (required by the module)";
            };
            stashdbApiKeyFile = lib.mkOption {
              type = lib.types.nullOr lib.types.path;
              default = null;
              description = "Host path to a file holding the StashDB API key, injected into config.yml at start";
            };
          };

        config = {
          systemd.services."container@stash" = {
            serviceConfig.MemoryHigh = lib.mkIf (
              config.containerConfigs.stash.memoryHigh != null
            ) config.containerConfigs.stash.memoryHigh;
          };

          containers.stash =
            let
              cfg = hostConfig.containerConfigs.stash;
            in
            {
              inherit (cfg) autoStart;
              ephemeral = true;
              timeoutStartSec = "5min";
              privateNetwork = true;
              inherit (cfg) hostBridge;
              localMacAddress = cfg.macAddress;
              inherit (cfg) localAddress;
              bindMounts =
                cfg.bindMounts
                # Secrets ride in as individual read-only file mounts rather than
                # an env file: the module reads each one's contents at start.
                // (lib.optionalAttrs (cfg.jwtSecretKeyFile != null) {
                  "/run/secrets/stash-jwt-secret" = {
                    hostPath = cfg.jwtSecretKeyFile;
                    isReadOnly = true;
                  };
                })
                // (lib.optionalAttrs (cfg.sessionStoreKeyFile != null) {
                  "/run/secrets/stash-session-key" = {
                    hostPath = cfg.sessionStoreKeyFile;
                    isReadOnly = true;
                  };
                })
                // (lib.optionalAttrs (cfg.stashdbApiKeyFile != null) {
                  "/run/secrets/stashdb-apikey" = {
                    hostPath = cfg.stashdbApiKeyFile;
                    isReadOnly = true;
                  };
                });

              config =
                { pkgs, lib, ... }:
                {
                  imports = [ (import ./_base.nix { inherit lib cfg; }) ];

                  services.stash = {
                    enable = true;
                    dataDir = "/var/lib/stash";
                    user = "stash";
                    group = "stash";
                    openFirewall = false;
                    # config.yml holds only paths, secrets and stash-boxes; the
                    # library itself (scenes, performers, tags, saved filters)
                    # lives in SQLite, so pinning the file costs no UI state.
                    mutableSettings = false;
                    # No login. Stash gates auth on HasCredentials(), which is
                    # `username != "" && pwHash != ""`, so an EMPTY password file
                    # turns it off: yq's `with(select($password != ""))` skips
                    # the key entirely. The file must exist and be non-null all
                    # the same — the module's ExecStartPre reads it
                    # unconditionally ($(< …)), and its assertion wants a
                    # username and a password file together. Access control is
                    # Caddy + Authelia.
                    passwordFile = pkgs.writeText "stash-no-password" "";
                    jwtSecretKeyFile = "/run/secrets/stash-jwt-secret";
                    sessionStoreKeyFile = "/run/secrets/stash-session-key";
                    settings = {
                      # Module default is localhost, which is unreachable across
                      # the bridge — the container perimeter is the firewall.
                      host = "0.0.0.0";
                      inherit (cfg) port;
                      # Present only to satisfy the assertion; with no password
                      # hash it never gates anything.
                      username = "admin";
                      parallel_tasks = cfg.parallelTasks;
                      stash = map (p: { path = p; }) cfg.libraryPaths;
                    };
                    # Resolved from HOST pkgs (see the let binding above): the
                    # container's own nixpkgs has no homelab overlay.
                    scrapers = [ scrapersPkg ];
                    plugins = [ pluginsPkg ];
                  };

                  systemd = {
                    services.stash = {
                      # Prepended so it wins over the module's plain python3.
                      path = lib.mkBefore [ pythonEnv ];

                      # StashDB credentials must NOT go through
                      # services.stash.settings: that attrset is rendered into a
                      # world-readable /nix/store file. The module keeps jwt and
                      # session keys out of it for the same reason but has no
                      # equivalent for stash-boxes, so patch the rendered
                      # config.yml instead. Appends to the module's own
                      # ExecStartPre, so it runs after the file is written.
                      serviceConfig.ExecStartPre = lib.mkIf (cfg.stashdbApiKeyFile != null) [
                        # Deliberately NOT `yq -i`: the unit runs under
                        # SystemCallFilter=~@privileged with PrivateUsers, and an
                        # in-place edit chowns its temp file to preserve ownership,
                        # which trips seccomp (SIGSYS). Write to stdout and rename,
                        # exactly as the module's own yq invocation does.
                        (pkgs.writeShellScript "stash-stashbox" ''
                          key=$(< /run/secrets/stashdb-apikey) \
                            ${lib.getExe pkgs.yq-go} '
                              .stash_boxes = [{
                                "name": "StashDB",
                                "endpoint": "https://stashdb.org/graphql",
                                "apikey": strenv(key)
                              }]
                            ' /var/lib/stash/config.yml > /var/lib/stash/config.yml.new
                          mv /var/lib/stash/config.yml.new /var/lib/stash/config.yml
                        '')
                      ];
                    };

                    # Scan → Identify on a timer, over the local GraphQL API (no
                    # auth to satisfy: there is no login). Both are queued jobs, so
                    # Stash runs them in submission order.
                    #
                    # scanGeneratePhashes is the point of the exercise: StashDB
                    # matches on perceptual hash, and without phashes identify
                    # degrades to guessing from filenames — which is exactly what
                    # scene filenames are bad for.
                    services.stash-tasks = lib.mkIf (cfg.taskInterval != null) {
                      description = "Stash scan and StashDB identify pass";
                      after = [ "stash.service" ];
                      requires = [ "stash.service" ];
                      path = [ pkgs.curl ];
                      serviceConfig = {
                        Type = "oneshot";
                        User = "stash";
                        Group = "stash";
                      };
                      script = ''
                        api() {
                          curl -sf -H 'Content-Type: application/json' \
                            -d "$1" http://127.0.0.1:${toString cfg.port}/graphql
                        }
                        api '{"query":"mutation { metadataScan(input: { scanGenerateCovers: true, scanGeneratePreviews: true, scanGenerateSprites: true, scanGeneratePhashes: true }) }"}'
                        api '{"query":"mutation { metadataIdentify(input: { sources: [{ source: { stash_box_endpoint: \"https://stashdb.org/graphql\" } }] }) }"}'
                      '';
                    };

                    timers.stash-tasks = lib.mkIf (cfg.taskInterval != null) {
                      description = "Periodic Stash scan and identify";
                      wantedBy = [ "timers.target" ];
                      timerConfig = {
                        OnCalendar = cfg.taskInterval;
                        Persistent = true;
                        RandomizedDelaySec = "5m";
                      };
                    };
                  };

                  users.users.stash = {
                    uid = lib.mkForce cfg.uid;
                    group = "stash";
                    isSystemUser = true;
                  };
                  users.groups.stash.gid = lib.mkForce cfg.gid;
                };
            };
        };
      };
  };
}
