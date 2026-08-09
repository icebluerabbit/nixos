/*
  metube — self-hosted YouTube downloader front-end using yt-dlp
  https://github.com/alexta69/metube
*/
_: {
  den.aspects.containers._.metube = {
    aspect-name = "containers.metube";
    includes = [ ];

    provides.to-hosts.nixos =
      { config, lib, ... }:
      {
        options.containerConfigs.metube = import ./_options.nix {
          inherit lib;
          defaultPort = 8081;
        };

        config = {
          systemd.services."container@metube" = lib.mkIf (config.containerConfigs.metube.memoryHigh != null) {
            serviceConfig.MemoryHigh = config.containerConfigs.metube.memoryHigh;
          };

          containers.metube =
            let
              cfg = config.containerConfigs.metube;
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
