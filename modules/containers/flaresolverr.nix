# flaresolverr — proxy server to bypass Cloudflare protection (indexer proxy)
# https://github.com/FlareSolverr/FlareSolverr
_: {
  den.aspects.containers._.flaresolverr = {
    aspect-name = "containers.flaresolverr";
    includes = [ ];

    provides.to-hosts.nixos =
      { config, lib, ... }:
      {
        options.containerConfigs.flaresolverr = import ./_options.nix {
          inherit lib;
          defaultPort = 8191;
        };

        config = {
          containers.flaresolverr =
            let
              cfg = config.containerConfigs.flaresolverr;
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
                  imports = [ (import ./_base.nix { inherit lib cfg; }) ];

                  services.flaresolverr = {
                    enable = true;
                    inherit (cfg) port;
                  };
                };
            };
        };
      };
  };
}
