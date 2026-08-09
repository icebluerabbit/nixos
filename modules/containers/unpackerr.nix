/*
  unpackerr — automated archive extraction daemon for downloader clients
  https://github.com/davidnewhall/unpackerr
*/
_: {
  den.aspects.containers._.unpackerr = {
    aspect-name = "containers.unpackerr";
    includes = [ ];

    provides.to-hosts.nixos =
      { config, lib, ... }:
      {
        options.containerConfigs.unpackerr =
          (import ./_options.nix {
            inherit lib;
            defaultUid = 5023;
            defaultGid = 5023;
          })
          // {
            settings = lib.mkOption {
              type = lib.types.attrs;
              default = { };
              description = "unpackerr settings (nix attrs → TOML); see services.unpackerr.settings";
            };
            envFile = lib.mkOption {
              type = lib.types.nullOr lib.types.path;
              default = null;
              description = "Host path to the env file with UN_*_API_KEY secrets, loaded via EnvironmentFile";
            };
          };

        config = {
          systemd.services."container@unpackerr" = {
            serviceConfig.MemoryHigh = lib.mkIf (
              config.containerConfigs.unpackerr.memoryHigh != null
            ) config.containerConfigs.unpackerr.memoryHigh;
          };

          containers.unpackerr =
            let
              cfg = config.containerConfigs.unpackerr;
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
                  "/run/secrets/unpackerr-env" = {
                    hostPath = cfg.envFile;
                    isReadOnly = true;
                  };
                });

              config =
                {
                  pkgs,
                  lib,
                  ...
                }:
                let
                  tomlFormat = pkgs.formats.toml { };
                  configFile = tomlFormat.generate "unpackerr.conf" cfg.settings;
                in
                {
                  imports = [
                    (import ./_base.nix {
                      inherit lib cfg;
                      hasPort = false;
                    })
                  ];

                  # nixpkgs has no unpackerr NixOS module — run the package directly.
                  # Polls each *arr queue, extracts completed archives in place so the
                  # *arr can import, then cleans up. Non-secret config (urls/paths) comes
                  # from the generated TOML; api_key is layered on via the env file
                  # (UN_<ARR>_0_API_KEY) so it never lands in the world-readable store.
                  systemd.services.unpackerr = {
                    description = "Unpackerr — extract *arr archives";
                    wantedBy = [ "multi-user.target" ];
                    after = [ "network.target" ];
                    serviceConfig = {
                      ExecStart = "${pkgs.unpackerr}/bin/unpackerr -c ${configFile}";
                      EnvironmentFile = lib.mkIf (cfg.envFile != null) "/run/secrets/unpackerr-env";
                      User = "unpackerr";
                      Group = "unpackerr";
                      Restart = "always";
                      RestartSec = 10;
                      StateDirectory = "unpackerr";
                    };
                  };

                  users.users.unpackerr = {
                    uid = lib.mkForce cfg.uid;
                    group = "unpackerr";
                    isSystemUser = true;
                    home = "/var/lib/unpackerr";
                  };
                  users.groups.unpackerr.gid = lib.mkForce cfg.gid;
                };
            };
        };
      };
  };
}
