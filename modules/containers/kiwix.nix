/*
  kiwix — Wikipedia and website offline reader/server
  https://www.kiwix.org
*/
_: {
  den.aspects.containers._.kiwix = {
    aspect-name = "containers.kiwix";
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
              # Resolved from host pkgs (the homelab overlay is not in the
              # container's nixpkgs); the store path pulls into the container
              # closure via ExecStart. The book list is argv, so adding a book is
              # a host-config change, not a script change.
              syncCmd = lib.escapeShellArgs (
                [
                  (lib.getExe pkgs.homelab.kiwix-sync)
                  "--mirror"
                  cfg.sync.mirror
                ]
                ++ lib.concatMap (b: [
                  "--book"
                  "${b.category}:${b.prefix}"
                ]) cfg.sync.books
              );
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
                { lib, ... }:
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
                          ExecStart = syncCmd;
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
