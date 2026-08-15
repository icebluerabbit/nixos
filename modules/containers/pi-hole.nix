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
            upstreams = lib.mkOption {
              type = lib.types.listOf lib.types.str;
              default = [ ];
              description = "Public forwarders Pi-hole sends non-internal queries to";
            };
            listeningMode = lib.mkOption {
              type = lib.types.enum [
                "LOCAL"
                "ALL"
                "BIND"
                "NONE"
              ];
              default = "LOCAL";
              description = ''
                Which sources FTL answers. LOCAL serves only Pi-hole's own subnet, so
                any cross-VLAN or VPN client needs ALL.
              '';
            };
            internalResolver = lib.mkOption {
              type = lib.types.nullOr lib.types.str;
              default = null;
              description = "Resolver that owns the internal zones; Pi-hole only forwards to it";
            };
            internalDomains = lib.mkOption {
              type = lib.types.listOf lib.types.str;
              default = [ ];
              description = "Domains forwarded verbatim to internalResolver rather than upstream";
            };
            reverseZones = lib.mkOption {
              type = lib.types.listOf lib.types.str;
              default = [ ];
              example = [ "10.0.0.0/8" ];
              description = "CIDRs whose PTR lookups go to internalResolver instead of upstream";
            };
            maxDBdays = lib.mkOption {
              type = lib.types.int;
              default = 91;
              description = "Days of query history FTL keeps before pruning its SQLite database";
            };
            lists = lib.mkOption {
              type = lib.types.listOf lib.types.attrs;
              default = [ ];
              description = ''
                Gravity allow/block lists. Leaving this empty masks pihole-ftl-setup,
                which is the only unit that ever builds gravity.db — so an empty list
                means Pi-hole blocks nothing at all.
              '';
            };
            gravityInterval = lib.mkOption {
              type = lib.types.str;
              default = "weekly";
              description = "systemd calendar spec for refreshing gravity list contents";
            };
            exporter = lib.mkOption {
              default = { };
              description = "prometheus-pihole-exporter sidecar (query/block stats via the FTL API)";
              type = lib.types.submodule {
                options = {
                  enable = lib.mkEnableOption "Pi-hole Prometheus exporter";
                  port = lib.mkOption {
                    type = lib.types.port;
                    default = 9617;
                    description = "Port the exporter listens on (scraped by the Vector sidecar)";
                  };
                };
              };
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
                {
                  config,
                  lib,
                  pkgs,
                  ...
                }:
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
                    inherit (cfg) lists;
                    settings = {
                      webserver.api.password = "";
                      dns = { inherit (cfg) upstreams listeningMode; };
                      database = { inherit (cfg) maxDBdays; };
                      # FTL hardcodes `local=/internal/` because ICANN reserved .internal
                      # as a special-use TLD, and a same-domain `server=` never wins
                      # against it regardless of ordering. dnsmasq resolves by longest
                      # domain match, so one entry per VLAN domain outranks the built-in
                      # and forwards it on. Pi-hole stores no internal records itself —
                      # internalResolver stays the sole authority.
                      misc.dnsmasq_lines = lib.optionals (cfg.internalResolver != null) (
                        map (domain: "server=/${domain}/${cfg.internalResolver}") cfg.internalDomains
                        ++ map (zone: "rev-server=${zone},${cfg.internalResolver}") cfg.reverseZones
                      );
                    };
                  };

                  # pihole-ftl-setup only runs `pihole -g` when gravity.db is absent, and
                  # nixpkgs ships no timer for it, so list *contents* would never refresh
                  # after the initial build.
                  systemd.services.pihole-gravity-refresh = lib.mkIf (cfg.lists != [ ]) {
                    description = "Refresh Pi-hole gravity blocklists";
                    after = [
                      "network-online.target"
                      "pihole-ftl.service"
                    ];
                    requires = [ "network-online.target" ];
                    startAt = cfg.gravityInterval;
                    serviceConfig = {
                      Type = "oneshot";
                      User = "pihole";
                      Group = "pihole";
                      ExecStart = "${lib.getExe config.services.pihole-ftl.piholePackage} -g";
                    };
                  };

                  # Prometheus exporter — reads the FTL v6 REST API (/api/stats/*). The
                  # nixpkgs exporter module is not used: it still passes the removed
                  # -pihole_api_token flag, puts any password on argv, and never forwards
                  # a bind address, so it would listen on 0.0.0.0.
                  systemd.services.pihole-exporter = lib.mkIf cfg.exporter.enable {
                    description = "Pi-hole Prometheus exporter";
                    after = [ "pihole-ftl.service" ];
                    wantedBy = [ "multi-user.target" ];
                    serviceConfig = {
                      ExecStart = "${lib.getExe pkgs.prometheus-pihole-exporter} -pihole_protocol http -pihole_hostname 127.0.0.1 -pihole_port ${toString cfg.webPort} -bind_addr 127.0.0.1 -port ${toString cfg.exporter.port}";
                      DynamicUser = true;
                      Restart = "on-failure";
                      RestartSec = 10;
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
