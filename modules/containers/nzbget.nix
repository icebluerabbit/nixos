/*
  nzbget — binary newsgrabber for Usenet downloads
  https://nzbget.net
*/
_: {
  den.aspects.containers._.nzbget = {
    aspect-name = "containers.nzbget";
    includes = [ ];

    provides.to-hosts.nixos =
      { config, lib, ... }:
      {
        options.containerConfigs.nzbget = import ./_options.nix {
          inherit lib;
          defaultPort = 6789;
        };

        config = {
          systemd.services."container@nzbget" = lib.mkIf (config.containerConfigs.nzbget.memoryHigh != null) {
            serviceConfig.MemoryHigh = config.containerConfigs.nzbget.memoryHigh;
          };

          containers.nzbget =
            let
              cfg = config.containerConfigs.nzbget;
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
