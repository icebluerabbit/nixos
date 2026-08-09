# modules/containers/prometheus.nix
_: {
  den.aspects.containers._.prometheus = {
    aspect-name = "containers.prometheus";
    includes = [ ];

    provides.to-hosts.nixos =
      { config, lib, ... }:
      {
        options.containerConfigs.prometheus = import ./_options.nix {
          inherit lib;
          defaultPort = 9090;
          defaultUid = 5300;
          defaultGid = 5300;
        };

        config = {
          systemd.services."container@prometheus" = {
            serviceConfig.MemoryHigh = lib.mkIf (
              config.containerConfigs.prometheus.memoryHigh != null
            ) config.containerConfigs.prometheus.memoryHigh;
          };

          containers.prometheus =
            let
              cfg = config.containerConfigs.prometheus;
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
                { ... }:
                {
                  imports = [
                    (import ./_base.nix { inherit lib cfg; })
                  ];

                  users.users.prometheus = {
                    uid = lib.mkForce cfg.uid;
                    group = "prometheus";
                    isSystemUser = true;
                  };
                  users.groups.prometheus.gid = lib.mkForce cfg.gid;

                  services.prometheus = {
                    enable = true;
                    inherit (cfg) port;
                    listenAddress = "0.0.0.0";
                    extraFlags = [
                      "--web.enable-remote-write-receiver"
                      "--web.enable-otlp-receiver"
                    ];
                  };
                };
            };
        };
      };
  };
}
