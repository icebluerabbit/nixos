# mosquitto — MQTT broker
_: {
  den.aspects.containers._.mosquitto = {
    aspect-name = "containers.mosquitto";
    includes = [ ];

    provides.to-hosts.nixos =
      { config, lib, ... }:
      let
        cfg = config.containerConfigs.mosquitto;
      in
      {
        options.containerConfigs.mosquitto = import ./_options.nix {
          inherit lib;
          defaultPort = 1883;
          defaultUid = 5020;
          defaultGid = 5020;
        };

        config.containers.mosquitto = {
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

              services.mosquitto = {
                enable = true;
                # Broker-wide (mosquitto rejects max_packet_size in a listener block):
                # 2.1 caps inbound packets at 10 KiB by default and refuses anything
                # larger, which silently dropped *arr webhook payloads (a multi-track
                # Lidarr Grab is 11-13 KB) — the publisher only sees "Cannot send packet
                # of size N". 256 KiB keeps the memory-exhaustion guard meaningful on an
                # anonymous broker.
                settings.max_packet_size = 262144;
                listeners = [
                  {
                    address = "0.0.0.0";
                    inherit (cfg) port;
                    settings.allow_anonymous = true;
                    # `#` does not match the special $SYS tree — grant it explicitly
                    # so the Vector metrics sidecar can read broker stats.
                    acl = [
                      "topic readwrite #"
                      "topic read $SYS/#"
                    ];
                  }
                ];
              };

              users.users.mosquitto = {
                uid = lib.mkForce cfg.uid;
                isSystemUser = lib.mkForce true;
              };
              users.groups.mosquitto.gid = lib.mkForce cfg.gid;
            };
        };
      };
  };
}
