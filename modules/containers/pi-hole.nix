/*
  pi-hole — network-wide ad blocking and local DNS resolver
  https://pi-hole.net
*/
_: {
  den.aspects.containers._.pi-hole = {
    aspect-name = "containers.pi-hole";
    includes = [ ];

    provides.to-hosts.nixos =
      { config, lib, ... }:
      let
        hostConfig = config;
      in
      {
        options.containerConfigs.pi-hole =
          (import ./_options.nix {
            inherit lib;
            defaultUid = 5013;
            defaultGid = 5013;
          })
          // {
            hostName = lib.mkOption {
              type = lib.types.str;
              default = "pi.hole";
              description = "Domain name for the website/dashboard";
            };
            dnsPort = lib.mkOption {
              type = lib.types.port;
              default = 53;
              description = "Pi-hole DNS port";
            };
            webPort = lib.mkOption {
              type = lib.types.port;
              default = 80;
              description = "Pi-hole Web UI port";
            };
          };

        config = {
          systemd.services."container@pi-hole" = {
            serviceConfig.MemoryHigh = lib.mkIf (
              config.containerConfigs.pi-hole.memoryHigh != null
            ) config.containerConfigs.pi-hole.memoryHigh;
          };

          containers.pi-hole =
            let
              cfg = hostConfig.containerConfigs.pi-hole;
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
                  imports = [
                    (import ./_base.nix {
                      inherit lib cfg;
                      hasPort = false;
                    })
                  ];

                  networking.firewall.allowedTCPPorts = [
                    cfg.dnsPort
                    cfg.webPort
                  ];
                  networking.firewall.allowedUDPPorts = [ cfg.dnsPort ];

                  services.pihole-ftl = {
                    enable = true;
                    settings = {
                      webserver.api.password = "";
                    };
                  };

                  services.pihole-web = {
                    enable = true;
                    inherit (cfg) hostName;
                    ports = [ cfg.webPort ];
                  };

                  users.users.pihole = {
                    uid = lib.mkForce cfg.uid;
                    isSystemUser = lib.mkForce false;
                    isNormalUser = true;
                  };
                  users.groups.pihole.gid = lib.mkForce cfg.gid;

                };
            };
        };
      };
  };
}
