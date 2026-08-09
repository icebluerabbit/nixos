# lazylibrarian — book/audiobook automation (readarr replacement) for Usenet/BitTorrent
# https://gitlab.com/LazyLibrarian/LazyLibrarian
{ inputs, ... }:
{
  flake-file.inputs.lazylibrarian-flake = {
    url = "github:icebluerabbit/lazylibrarian-flake/dev";
    inputs.nixpkgs.follows = "nixpkgs";
  };

  den.aspects.containers._.lazylibrarian = {
    aspect-name = "containers.lazylibrarian";
    includes = [ ];

    provides.to-hosts.nixos =
      { config, lib, ... }:
      let
        hostConfig = config;
      in
      {
        options.containerConfigs.lazylibrarian =
          (import ./_options.nix {
            inherit lib;
            defaultPort = 5299;
            defaultUid = 5030;
            defaultGid = 5030;
          })
          // {
            envFile = lib.mkOption {
              type = lib.types.nullOr lib.types.path;
              default = null;
              description = "Host path to the environment file containing secrets to be mounted and loaded";
            };
            configuratarr = lib.mkOption {
              default = { };
              description = "Declarative configuratarr sync for this LazyLibrarian instance";
              type = lib.types.submodule {
                options = {
                  enable = lib.mkEnableOption "configuratarr declarative sync";
                  settings = lib.mkOption {
                    type = lib.types.attrs;
                    default = { };
                    description = "configuratarr settings (verbatim); each top-level key is a named instance";
                  };
                  prune = lib.mkOption {
                    type = lib.types.bool;
                    default = false;
                    description = "Delete server-side resources absent from settings";
                  };
                  waitForHealthy = lib.mkOption {
                    type = lib.types.bool;
                    default = true;
                    description = "Poll each app's health endpoint before syncing";
                  };
                  waitTimeout = lib.mkOption {
                    type = lib.types.ints.unsigned;
                    default = 120;
                    description = "Max seconds to wait per service when waitForHealthy";
                  };
                };
              };
            };
          };

        config = {
          systemd.services."container@lazylibrarian" = {
            serviceConfig.MemoryHigh = lib.mkIf (
              config.containerConfigs.lazylibrarian.memoryHigh != null
            ) config.containerConfigs.lazylibrarian.memoryHigh;
          };

          containers.lazylibrarian =
            let
              cfg = hostConfig.containerConfigs.lazylibrarian;
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
                // (lib.optionalAttrs (cfg.envFile != null) {
                  "/run/secrets/lazylibrarian-env" = {
                    hostPath = cfg.envFile;
                    isReadOnly = true;
                  };
                });

              config =
                { lib, ... }:
                {
                  imports = [
                    (import ./_base.nix { inherit lib cfg; })
                    inputs.configuratarr.nixosModules.default
                    inputs.lazylibrarian-flake.nixosModules.default
                  ];

                  # Declarative config sync. Module self-provides its package and
                  # orders after lazylibrarian.service; waitForHealthy polls the
                  # API before syncing. Auth = the seeded api key (below).
                  services.configuratarr = lib.mkIf cfg.configuratarr.enable {
                    enable = true;
                    inherit (cfg.configuratarr)
                      settings
                      prune
                      waitForHealthy
                      waitTimeout
                      ;
                  };

                  services.lazylibrarian = {
                    enable = true;
                    inherit (cfg) port;
                    environmentFile = lib.mkIf (cfg.envFile != null) "/run/secrets/lazylibrarian-env";
                    settings.General.api_enabled = 1;
                    secretSettings."General.api_key" = "LAZYLIBRARIAN_API_KEY";
                  };

                  users.users.lazylibrarian.uid = lib.mkForce cfg.uid;
                  users.groups.lazylibrarian.gid = lib.mkForce cfg.gid;

                  # Feed the shared api-key env into the sync oneshot so
                  # ${env.LAZYLIBRARIAN_API_KEY} / ${env.PROWLARR_API_KEY} resolve.
                  systemd.services.configuratarr = lib.mkIf cfg.configuratarr.enable {
                    serviceConfig.EnvironmentFile = lib.mkIf (cfg.envFile != null) "/run/secrets/lazylibrarian-env";
                  };
                };
            };
        };
      };
  };
}
