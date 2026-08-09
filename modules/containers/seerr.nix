/*
  seerr — request management and media discovery tool for Jellyfin/Plex (Overseerr fork)
  https://github.com/fallenbagel/jellyseerr
*/
_: {
  den.aspects.containers._.seerr = {
    aspect-name = "containers.seerr";
    includes = [ ];

    provides.to-hosts.nixos =
      { config, lib, ... }:
      {
        options.containerConfigs.seerr = import ./_options.nix {
          inherit lib;
          defaultPort = 5055;
        };

        config = {
          systemd.services."container@seerr" = lib.mkIf (config.containerConfigs.seerr.memoryHigh != null) {
            serviceConfig.MemoryHigh = config.containerConfigs.seerr.memoryHigh;
          };

          containers.seerr =
            let
              cfg = config.containerConfigs.seerr;
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
