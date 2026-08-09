/*
  nzbhydra — meta search engine for Usenet indexers
  https://github.com/theotherp/nzbhydra2
*/
_: {
  den.aspects.containers._.nzbhydra = {
    aspect-name = "containers.nzbhydra";
    includes = [ ];

    provides.to-hosts.nixos =
      { config, lib, ... }:
      {
        options.containerConfigs.nzbhydra = import ./_options.nix {
          inherit lib;
          defaultPort = 5076;
        };

        config = {
          systemd.services."container@nzbhydra" = lib.mkIf (
            config.containerConfigs.nzbhydra.memoryHigh != null
          ) { serviceConfig.MemoryHigh = config.containerConfigs.nzbhydra.memoryHigh; };

          containers.nzbhydra =
            let
              cfg = config.containerConfigs.nzbhydra;
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
