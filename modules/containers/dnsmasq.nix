# modules/containers/dnsmasq.nix
_: {
  den.aspects.containers._.dnsmasq = {
    aspect-name = "containers.dnsmasq";
    includes = [ ];

    provides.to-hosts.nixos =
      { config, lib, ... }:
      {
        options.containerConfigs.dnsmasq =
          (import ./_options.nix {
            inherit lib;
            defaultPort = 53;
          })
          // {
            dnsRecords = lib.mkOption {
              type = lib.types.attrsOf lib.types.str;
              default = { };
              description = "DNS records mapping subdomains/hosts to target IP addresses";
            };
            exporter = lib.mkOption {
              default = { };
              description = "prometheus-dnsmasq-exporter sidecar (cache/query stats via CHAOS TXT)";
              type = lib.types.submodule {
                options = {
                  enable = lib.mkEnableOption "dnsmasq Prometheus exporter";
                  port = lib.mkOption {
                    type = lib.types.port;
                    default = 9153;
                    description = "Port the exporter listens on (scraped by the Vector sidecar)";
                  };
                };
              };
            };
          };

        config = {
          systemd.services."container@dnsmasq" = {
            serviceConfig.MemoryHigh = lib.mkIf (
              config.containerConfigs.dnsmasq.memoryHigh != null
            ) config.containerConfigs.dnsmasq.memoryHigh;
          };

          containers.dnsmasq =
            let
              cfg = config.containerConfigs.dnsmasq;
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
                {
                  pkgs,
                  lib,
                  ...
                }:
                {
                  imports = [
                    (import ./_base.nix { inherit lib cfg; })
                  ];

                  # Internal container firewall (allow DNS UDP traffic in addition to TCP)
                  networking.firewall.allowedUDPPorts = [ cfg.port ];

                  services.dnsmasq = {
                    enable = true;
                    settings = {
                      inherit (cfg) port;
                      # Parent forwarders: Gateway first (for DHCP hostnames), then Cloudflare
                      server = lib.optional (cfg.gateway != null) cfg.gateway ++ [
                        "1.1.1.1"
                        "1.0.0.1"
                      ];
                      # Configure static local DNS records
                      address = lib.mapAttrsToList (domain: ip: "/${domain}/${ip}") cfg.dnsRecords;
                      # RFC1918 reverse lookups go to the gateway only: it is the sole
                      # holder of DHCP-lease PTR records, and public forwarders answer
                      # these with an instant NXDOMAIN that wins the race against it
                      # (gateway 5ms vs Cloudflare 6ms) and then sticks for neg-ttl.
                      rev-server = lib.optional (cfg.gateway != null) "10.0.0.0/8,${cfg.gateway}";

                      # The 150-entry default evicted unexpired entries constantly and
                      # forwarded ~45% of all queries upstream, so every upstream latency
                      # spike surfaced as a client-side timeout.
                      cache-size = 10000;
                      min-cache-ttl = 120;
                      neg-ttl = 60;
                      # The 150 default forward slots exhaust during a spike, after which
                      # dnsmasq drops queries silently.
                      dns-forward-max = 1000;
                      # No bogus-priv: the gateway is the only source of DHCP-lease PTR
                      # records, so answering RFC1918 reverse lookups locally with NXDOMAIN
                      # breaks them. neg-ttl above already stops the repeat round-trips,
                      # which was the only real cost.
                      domain-needed = true;
                    };
                  };

                  # Critical-path resolver: protect it from noisy co-tenant containers
                  # (e.g. the qbittorrent seedbox) starving it of CPU on a loaded host.
                  # Nice is host-global, so a -10 dnsmasq preempts nice-0 torrent threads
                  # regardless of cgroup; high CPUWeight biases the container's internal
                  # cgroup; OOMScoreAdjust keeps it off the OOM shortlist.
                  systemd.services.dnsmasq.serviceConfig = {
                    Nice = -10;
                    CPUWeight = 9000;
                    OOMScoreAdjust = -800;
                  };

                  # Prometheus exporter — queries dnsmasq's CHAOS TXT stat records
                  # (cachesize.bind, hits.bind, …) on the local DNS port.
                  systemd.services.dnsmasq-exporter = lib.mkIf cfg.exporter.enable {
                    description = "dnsmasq Prometheus exporter";
                    after = [ "dnsmasq.service" ];
                    wantedBy = [ "multi-user.target" ];
                    serviceConfig = {
                      ExecStart = "${pkgs.prometheus-dnsmasq-exporter}/bin/dnsmasq_exporter --dnsmasq 127.0.0.1:${toString cfg.port} --listen 127.0.0.1:${toString cfg.exporter.port}";
                      DynamicUser = true;
                      Restart = "on-failure";
                      RestartSec = 10;
                    };
                  };
                };
            };
        };
      };
  };
}
