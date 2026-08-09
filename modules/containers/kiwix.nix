/*
  kiwix — Wikipedia and website offline reader/server
  https://www.kiwix.org
*/
_: {
  den.aspects.containers._.kiwix = {
    aspect-name = "containers.kiwix";
    includes = [ ];

    provides.to-hosts.nixos =
      { config, lib, ... }:
      let
        hostConfig = config;
      in
      {
        options.containerConfigs.kiwix =
          (import ./_options.nix {
            inherit lib;
            defaultPort = 8080;
            defaultUid = 5010;
            defaultGid = 5010;
          })
          // {
            sync = {
              enable = lib.mkOption {
                type = lib.types.bool;
                default = false;
                description = "Enable automatic monthly sync of ZIM files from a Kiwix mirror";
              };
              mirror = lib.mkOption {
                type = lib.types.str;
                default = "rsync://mirrors.dotsrc.org/kiwix/zim";
                description = "Base rsync URL of the Kiwix mirror (path up to /zim)";
              };
              books = lib.mkOption {
                type = lib.types.listOf (
                  lib.types.submodule {
                    options = {
                      category = lib.mkOption {
                        type = lib.types.str;
                        description = "ZIM category subdirectory (e.g. wikipedia, wiktionary)";
                      };
                      prefix = lib.mkOption {
                        type = lib.types.str;
                        description = "ZIM filename prefix without date/extension (e.g. wikipedia_en_all_maxi)";
                      };
                    };
                  }
                );
                default = [ ];
                description = "ZIM books to keep synced";
              };
            };
          };

        config = {
          systemd.services."container@kiwix" = {
            serviceConfig.MemoryHigh = lib.mkIf (
              config.containerConfigs.kiwix.memoryHigh != null
            ) config.containerConfigs.kiwix.memoryHigh;
          };

          containers.kiwix =
            let
              cfg = hostConfig.containerConfigs.kiwix;
              inherit (cfg) port;
            in
            {
              inherit (cfg) autoStart;
              ephemeral = true;
              timeoutStartSec = "5min";
              privateNetwork = true;
              inherit (cfg) hostBridge;
              localMacAddress = cfg.macAddress;
              inherit (cfg) localAddress;
              inherit (cfg) bindMounts;

              config =
                {
                  pkgs,
                  lib,
                  ...
                }:
                let
                  syncScript = pkgs.writeShellApplication {
                    name = "kiwix-sync";
                    runtimeInputs = [ pkgs.rsync ];
                    text = ''
                      ZIM_DIR=/zim

                      sync_book() {
                        local category="$1"
                        local prefix="$2"

                        echo "[$prefix] Querying mirror for latest..."
                        local latest
                        latest=$(rsync --list-only "$MIRROR/$category/$prefix"'_*.zim' 2>/dev/null \
                          | awk '{print $NF}' \
                          | grep -v '\.part$' \
                          | sort -V \
                          | tail -1)

                        if [[ -z "$latest" ]]; then
                          echo "[$prefix] No files found on mirror, skipping"
                          return 0
                        fi

                        if [[ -f "$ZIM_DIR/$latest" ]]; then
                          echo "[$prefix] Already have $latest, nothing to do"
                          find "$ZIM_DIR" -name "$prefix"'_*.zim.part' -delete
                          return 0
                        fi

                        echo "[$prefix] Syncing $latest..."
                        find "$ZIM_DIR" -name "$prefix"'_*.zim' -delete
                        find "$ZIM_DIR" -name "$prefix"'_*.zim.part' -not -name "$latest"'.part' -delete

                        rsync -av --partial --progress "$MIRROR/$category/$latest" "$ZIM_DIR/$latest"'.part'
                        mv "$ZIM_DIR/$latest"'.part' "$ZIM_DIR/$latest"
                        echo "[$prefix] Done: $latest"
                      }

                      ${lib.concatMapStrings (book: ''
                        sync_book ${lib.escapeShellArg book.category} ${lib.escapeShellArg book.prefix}
                      '') cfg.sync.books}

                      echo "Kiwix ZIM sync completed."
                    '';
                  };
                in
                {
                  imports = [
                    (import ./_base.nix { inherit lib cfg; })
                  ];

                  services.kiwix-serve = {
                    enable = true;
                    inherit port;
                    libraryPath = "/zim";
                  };

                  users = {
                    users.kiwix = {
                      uid = lib.mkForce cfg.uid;
                      isSystemUser = lib.mkForce false;
                      isNormalUser = true;
                    };
                    groups.kiwix.gid = lib.mkForce cfg.gid;
                  };

                  systemd = {
                    services = {
                      kiwix-serve.serviceConfig = {
                        User = "kiwix";
                        Group = "kiwix";
                      };
                      kiwix-sync = lib.mkIf cfg.sync.enable {
                        description = "Sync Kiwix ZIM files from mirror";
                        wants = [ "network-online.target" ];
                        after = [ "network-online.target" ];
                        serviceConfig = {
                          Type = "oneshot";
                          User = "kiwix";
                          Group = "kiwix";
                          TimeoutStartSec = "infinity";
                          Restart = "on-failure";
                          RestartSec = "1800";
                          Environment = "MIRROR=${cfg.sync.mirror}";
                          ExecStart = lib.getExe syncScript;
                        };
                      };
                    };
                    timers.kiwix-sync = lib.mkIf cfg.sync.enable {
                      wantedBy = [ "timers.target" ];
                      timerConfig = {
                        OnCalendar = "monthly";
                        Persistent = true;
                      };
                    };
                  };

                };
            };
        };
      };
  };
}
