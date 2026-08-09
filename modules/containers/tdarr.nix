/*
  tdarr — distributed transcode orchestrator for media libraries
  https://tdarr.io
*/
_: {
  den.aspects.containers._.tdarr = {
    aspect-name = "containers.tdarr";
    includes = [ ];

    provides.to-hosts.nixos =
      { config, lib, ... }:
      {
        options.containerConfigs.tdarr = import ./_options.nix {
          inherit lib;
          defaultPort = 8265;
        };

        config = {
          systemd.services."container@tdarr" = lib.mkIf (config.containerConfigs.tdarr.memoryHigh != null) {
            serviceConfig.MemoryHigh = config.containerConfigs.tdarr.memoryHigh;
          };

          containers.tdarr =
            let
              cfg = config.containerConfigs.tdarr;
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
              config = _: (import ./_base.nix { inherit lib cfg; });
            };
        };
      };
  };
}
