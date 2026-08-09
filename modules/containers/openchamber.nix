# openchamber — AI coding agent web UI
# https://github.com/icebluerabbit/openchamber-flake
{
  inputs,
  ...
}:
{
  flake-file.inputs = {
    openchamber-flake = {
      url = "github:icebluerabbit/openchamber-flake/dev";
      inputs.nixpkgs.follows = "nixpkgs";
    };
  };

  den.aspects.containers._.openchamber = {
    aspect-name = "containers.openchamber";
    includes = [ ];

    provides.to-hosts.nixos =
      { config, lib, ... }:
      {
        options.containerConfigs.openchamber =
          (import ./_options.nix {
            inherit lib;
            defaultPort = 3000;
            defaultUid = 5016;
            defaultGid = 5016;
          })
          // {
            opencodeHost = lib.mkOption {
              type = lib.types.str;
              default = "";
              description = "Base URL of the external OpenCode server (e.g. http://10.10.140.7)";
            };
            opencodePort = lib.mkOption {
              type = lib.types.nullOr lib.types.port;
              default = null;
              description = "Port of the external OpenCode server";
            };
            uiPasswordFile = lib.mkOption {
              type = lib.types.nullOr lib.types.path;
              default = null;
              description = "Path to file containing the UI password";
            };
            settings = lib.mkOption {
              type = lib.types.attrs;
              default = { };
              description = "Freeform attrs merged into services.openchamber.settings";
            };
          };

        config = {
          systemd.services."container@openchamber" = {
            serviceConfig.MemoryHigh = lib.mkIf (
              config.containerConfigs.openchamber.memoryHigh != null
            ) config.containerConfigs.openchamber.memoryHigh;
          };

          containers.openchamber =
            let
              cfg = config.containerConfigs.openchamber;
              openchamberModule = inputs.openchamber-flake.nixosModules.openchamber;
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
                { pkgs, lib, ... }:
                {
                  imports = [
                    (import ./_base.nix { inherit lib cfg; })
                    openchamberModule
                  ];

                  users.users.openchamber.uid = lib.mkForce cfg.uid;
                  users.groups.openchamber.gid = lib.mkForce cfg.gid;

                  services.openchamber = {
                    enable = true;
                    package = inputs.openchamber-flake.packages.${pkgs.system}.openchamber;
                    inherit (cfg) port;
                    host = "0.0.0.0";
                    uiPasswordFile = lib.mkIf (cfg.uiPasswordFile != null) cfg.uiPasswordFile;
                    inherit (cfg) settings;
                    extraEnvironment = {
                      OPENCHAMBER_ALLOW_UNAUTHENTICATED_LAN = "true";
                    };
                    opencode = {
                      host = lib.mkIf (cfg.opencodeHost != "") (
                        if cfg.opencodePort != null then
                          "${cfg.opencodeHost}:${toString cfg.opencodePort}"
                        else
                          cfg.opencodeHost
                      );
                      skipStart = true;
                    };
                  };
                };
            };
        };
      };
  };
}
