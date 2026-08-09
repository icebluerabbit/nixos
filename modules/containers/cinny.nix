_: {
  den.aspects.containers._.cinny = {
    aspect-name = "containers.cinny";
    includes = [ ];

    provides.to-hosts.nixos =
      { config, lib, ... }:
      let
        hostConfig = config;
      in
      {
        options.containerConfigs.cinny =
          (import ./_options.nix {
            inherit lib;
            defaultPort = 80;
            defaultUid = 5025;
            defaultGid = 5025;
          })
          // {
            homeserverDomain = lib.mkOption {
              type = lib.types.str;
              description = "Matrix homeserver domain for Cinny client config (e.g. tuwunel.caddy.apps.internal)";
            };
          };

        config =
          let
            cfg = hostConfig.containerConfigs.cinny;
          in
          {
            containers.cinny = {
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
                let
                  cinnyConfig = pkgs.writeText "cinny-config.json" (
                    builtins.toJSON {
                      defaultHomeserver = 0;
                      homeserverList = [ cfg.homeserverDomain ];
                      allowCustomHomeservers = false;
                    }
                  );
                in
                {
                  imports = [ (import ./_base.nix { inherit lib cfg; }) ];

                  services.nginx = {
                    enable = true;
                    virtualHosts."cinny" = {
                      listen = [
                        {
                          addr = "0.0.0.0";
                          inherit (cfg) port;
                          ssl = false;
                        }
                      ];
                      root = "${pkgs.cinny}";
                      locations."/" = {
                        tryFiles = "$uri $uri/ /index.html";
                      };
                      locations."= /config.json" = {
                        alias = "${cinnyConfig}";
                      };
                    };
                  };

                  users.users.nginx.uid = lib.mkForce cfg.uid;
                  users.groups.nginx.gid = lib.mkForce cfg.gid;
                };
            };
          };
      };
  };
}
