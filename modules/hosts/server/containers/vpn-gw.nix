{ den, ... }:
{
  den.aspects.hosts._.server._.containers._.vpn-gw = {
    aspect-name = "hosts.server.containers.vpn-gw";
    includes = [
      den.aspects.containers._.vpn-gw
      den.aspects.infra._.clients._.container-vpn-gw
      den.aspects.infra._.clients._.container-loki
      den.aspects.infra._.clients._.container-prometheus
      den.aspects.infra._.clients._.container-tempo
      den.aspects.infra._.networks._.download
      den.aspects.hosts._.server._.security
      den.aspects.hosts._.server._.networking._.vlans._.download
    ];

    nixos =
      { config, ... }:
      {
        # Kernel WireGuard (host-global module; the container creates wg links in its
        # own netns with CAP_NET_ADMIN).
        boot.kernelModules = [ "wireguard" ];

        # The per-tunnel bridges (br-vpnN) carry no host IP, so nothing else pulls their
        # netdev units in — require them so they're created before the container enslaves
        # its veths (post-start would otherwise fail: bridge doesn't exist).
        systemd.services."container@vpn-gw" = {
          before = [ "container-wave-foundation.target" ];
          after = [
            "br-vpn1-netdev.service"
            "br-vpn2-netdev.service"
            "br-vpn3-netdev.service"
            "br-vpn4-netdev.service"
            "br-vpn5-netdev.service"
            "br-vpn6-netdev.service"
            "br-vpn7-netdev.service"
            "br-vpn8-netdev.service"
            "br-vpn9-netdev.service"
            "br-vpn10-netdev.service"
          ];
          requires = [
            "br-vpn1-netdev.service"
            "br-vpn2-netdev.service"
            "br-vpn3-netdev.service"
            "br-vpn4-netdev.service"
            "br-vpn5-netdev.service"
            "br-vpn6-netdev.service"
            "br-vpn7-netdev.service"
            "br-vpn8-netdev.service"
            "br-vpn9-netdev.service"
            "br-vpn10-netdev.service"
          ];
        };

        # Per-tunnel wg-quick confs (PrivateKey/PresharedKey/Endpoint/Address +
        # Table=20N + PersistentKeepalive). Provider-identifying values live only here,
        # encrypted — never in git or the store.
        sops.secrets = {
          "wg1-conf" = { };
          "wg2-conf" = { };
          "wg3-conf" = { };
          "wg4-conf" = { };
          "wg5-conf" = { };
          "wg6-conf" = { };
          "wg7-conf" = { };
          "wg8-conf" = { };
          "wg9-conf" = { };
          "wg10-conf" = { };
        };

        containerConfigs."vpn-gw" = {
          lldpd.enable = true;
          hostBridge = "br-download";
          macAddress = config.infra.clients.container-vpn-gw.mac;
          localAddress = "${config.infra.clients.container-vpn-gw.ip}/${toString config.infra.vlans.download.subnetPrefix}";
          gateway = config.infra.vlans.download.staticAddresses.gateway;
          nameservers = [ "1.1.1.1" ];

          # Tunnel pool. Each tunnel has a p2p veth on br-vpnN (192.168.N.1 = vpn-gw); tunnels
          # are unowned until claimed. A claiming client passes its own veth IP (192.168.N.2) at
          # dnat() time as the inbound DNAT target — the broker doesn't pre-know client IPs. Table
          # 20N matches Table= in the conf. All tunnels are Proton (natmap, gateway 10.2.0.1).
          tunnels = [
            {
              name = "wg1";
              conf = config.sops.secrets."wg1-conf".path;
              bridge = "br-vpn1";
              ip = "192.168.1.1";
              table = 201;
              portForward = {
                type = "natmap";
                gateway = "10.2.0.1";
              };
            }
            {
              name = "wg2";
              conf = config.sops.secrets."wg2-conf".path;
              bridge = "br-vpn2";
              ip = "192.168.2.1";
              table = 202;
              portForward = {
                type = "natmap";
                gateway = "10.2.0.1";
              };
            }
            {
              name = "wg3";
              conf = config.sops.secrets."wg3-conf".path;
              bridge = "br-vpn3";
              ip = "192.168.3.1";
              table = 203;
              portForward = {
                type = "natmap";
                gateway = "10.2.0.1";
              };
            }
            {
              name = "wg4";
              conf = config.sops.secrets."wg4-conf".path;
              bridge = "br-vpn4";
              ip = "192.168.4.1";
              table = 204;
              portForward = {
                type = "natmap";
                gateway = "10.2.0.1";
              };
            }
            {
              name = "wg5";
              conf = config.sops.secrets."wg5-conf".path;
              bridge = "br-vpn5";
              ip = "192.168.5.1";
              table = 205;
              portForward = {
                type = "natmap";
                gateway = "10.2.0.1";
              };
            }
            {
              name = "wg6";
              conf = config.sops.secrets."wg6-conf".path;
              bridge = "br-vpn6";
              ip = "192.168.6.1";
              table = 206;
              portForward = {
                type = "natmap";
                gateway = "10.2.0.1";
              };
            }
            {
              name = "wg7";
              conf = config.sops.secrets."wg7-conf".path;
              bridge = "br-vpn7";
              ip = "192.168.7.1";
              table = 207;
              portForward = {
                type = "natmap";
                gateway = "10.2.0.1";
              };
            }
            {
              name = "wg8";
              conf = config.sops.secrets."wg8-conf".path;
              bridge = "br-vpn8";
              ip = "192.168.8.1";
              table = 208;
              portForward = {
                type = "natmap";
                gateway = "10.2.0.1";
              };
            }
            {
              name = "wg9";
              conf = config.sops.secrets."wg9-conf".path;
              bridge = "br-vpn9";
              ip = "192.168.9.1";
              table = 209;
              portForward = {
                type = "natmap";
                gateway = "10.2.0.1";
              };
            }
            {
              name = "wg10";
              conf = config.sops.secrets."wg10-conf".path;
              bridge = "br-vpn10";
              ip = "192.168.10.1";
              table = 210;
              portForward = {
                type = "natmap";
                gateway = "10.2.0.1";
              };
            }
          ];

          controlPort = config.infra.clients.container-vpn-gw.ports.control;

          nodeExporterPort = config.infra.clients.container-vpn-gw.ports.nodeExporter;
          wireguardExporterPort = config.infra.clients.container-vpn-gw.ports.wireguardExporter;

          logging = {
            enable = true;
            lokiAddress = "http://${config.infra.clients.container-loki.ip}:${toString config.infra.clients.container-loki.ports.default}";
            prometheusAddress = "http://${config.infra.clients.container-prometheus.ip}:${toString config.infra.clients.container-prometheus.ports.default}/api/v1/write";
            tempoAddress = "http://${config.infra.clients.container-tempo.ip}:${toString config.infra.clients.container-tempo.ports.otlp-http}/v1/traces";
            prometheusInputs = [
              "vpngw_node"
              "vpngw_wireguard"
              "vpngw_broker"
            ];
            extraSettings = {
              sources = {
                vpngw_node = {
                  type = "prometheus_scrape";
                  endpoints = [
                    "http://127.0.0.1:${toString config.infra.clients.container-vpn-gw.ports.nodeExporter}/metrics"
                  ];
                  scrape_interval_secs = 15;
                };
                # broker allocation/health/port/exit-ip metrics (replaces the old exit-ip
                # textfile + qbt's net-config log-line VRL).
                vpngw_broker = {
                  type = "prometheus_scrape";
                  endpoints = [
                    "http://127.0.0.1:${toString config.infra.clients.container-vpn-gw.ports.control}/export"
                  ];
                  scrape_interval_secs = 30;
                };
                vpngw_wireguard = {
                  type = "prometheus_scrape";
                  endpoints = [
                    "http://127.0.0.1:${toString config.infra.clients.container-vpn-gw.ports.wireguardExporter}/metrics"
                  ];
                  scrape_interval_secs = 30;
                };
              };
            };
          };
        };
      };
  };
}
