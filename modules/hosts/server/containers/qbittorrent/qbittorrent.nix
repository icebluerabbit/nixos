# qbittorrent — host configuration override for qbittorrent container
{ den, ... }:
{
  den.aspects.hosts._.server._.containers._.qbittorrent = {
    aspect-name = "hosts.server.containers.qbittorrent";
    includes = [
      den.aspects.containers._.qbittorrent
      den.aspects.containers._.vpn-gw
      den.aspects.infra._.clients._.container-vpn-gw
      den.aspects.infra._.clients._.container-qbittorrent
      den.aspects.infra._.clients._.container-cross-seed
      den.aspects.infra._.clients._.container-home-assistant
      den.aspects.infra._.clients._.container-mosquitto
      den.aspects.infra._.clients._.container-dns
      den.aspects.infra._.networks._.download
      den.aspects.infra._.networks._.apps
      den.aspects.infra._.networks._.media
      den.aspects.infra._.networks._.iot
      den.aspects.hosts._.server._.networking._.vlans._.download
      den.aspects.infra._.clients._.container-loki
      den.aspects.infra._.clients._.container-prometheus
      den.aspects.infra._.clients._.container-tempo
    ];

    nixos =
      {
        config,
        pkgs,
        lib,
        ...
      }:
      let
        cfg = config.containerConfigs.qbittorrent;
        categories = {
          tv = "/mnt/media/complete/tv";
          movies = "/mnt/media/complete/movies";
          books = "/mnt/media/complete/books";
          audiobooks = "/mnt/media/complete/audiobooks";
          music = "/mnt/media/complete/music";
          misc = "/mnt/media/complete/misc";
          # seasonpackarr's preImportPath — packs it assembles from episodes
          # already held here are added under this category by autobrr, so the
          # save path must match what seasonpackarr hardlinked into. Kept out of
          # Sonarr's `tv` category on purpose: Sonarr would try to import these
          # packs and reject them as "not an upgrade", which turns into strikes
          # once cleanuparr's failed_import rule leaves phase 0.
          seasonpacks = "/mnt/media/complete/seasonpacks";
        };
        categoriesJson = pkgs.writeText "categories.json" (
          builtins.toJSON (lib.mapAttrs (_: sp: { save_path = sp; }) categories)
        );
        cidr = vlan: "${vlan.subnet}/${toString vlan.subnetPrefix}";
        # qbt AutoRun hooks POST here on loopback → local Vector sidecar → MQTT
        # (homelab/notify/qbittorrent); HA consumes via an mqtt trigger. No webhook to HA.
        haWebhook = "http://127.0.0.1:9001/";
        onAdded = import ./_on_added.nix {
          inherit pkgs lib;
          webhook = haWebhook;
        };
        onFinished = import ./_on_finished.nix {
          inherit pkgs lib;
          webhook = haWebhook;
          crossSeed = config.infra.clients.container-cross-seed;
        };
        # qbt's own veth IP on each tunnel bridge (qbt owns these; the broker client passes the
        # right one at dnat() time). Everything else (bridge/gateway/table/veth) is derived from
        # the vpn-gw tunnel pool — no duplication.
        qbtVpnIps = {
          wg1 = "192.168.1.2";
          wg2 = "192.168.2.2";
          wg3 = "192.168.3.2";
          wg4 = "192.168.4.2";
          wg5 = "192.168.5.2";
          wg6 = "192.168.6.2";
          wg7 = "192.168.7.2";
          wg8 = "192.168.8.2";
          wg9 = "192.168.9.2";
          wg10 = "192.168.10.2";
        };
        qbtVpnPool = map (t: {
          tunnel = t.name;
          veth = lib.removePrefix "br-" t.bridge; # br-vpn1 -> vpn1
          ip = qbtVpnIps.${t.name};
          gateway = t.ip; # vpn-gw's side of the veth
          inherit (t) table bridge;
        }) config.containerConfigs."vpn-gw".tunnels;
      in
      {
        # cross-seed apikey for the on-finished webhook. mode 0444: the qbt AutoRun
        # program runs as the qbittorrent user and must read this bind-mounted secret
        # (default 0400 root → Permission denied). privateUsers=no, so container uid
        # == host uid; 0444 lets the container user read it.
        sops.secrets.cross-seed-apikey.mode = "0444";
        # mam_id seed cookie for the MAM dynamic-seedbox re-lock (read by the root keepalive).
        sops.secrets.mam-vpn-id.mode = "0444";

        containerConfigs.qbittorrent = {
          lldpd.enable = true;
          hostBridge = "br-download";
          macAddress = config.infra.clients.container-qbittorrent.mac;
          localAddress = "${config.infra.clients.container-qbittorrent.ip}/${toString config.infra.vlans.download.subnetPrefix}";
          # Default route = LAN router (webui/arr/NFS normal). Torrent traffic is bound
          # to a vpnN interface and routed out the VPN separately (no bypass routes).
          gateway = config.infra.vlans.download.staticAddresses.gateway;
          webuiPort = config.infra.clients.container-qbittorrent.ports.webui;
          profileDir = "/var/lib/qbittorrent";
          # Tunnel broker on vpn-gw. qbt's veths + egress routing are built from vpnPool
          # (derived above from the pool + qbt's own IPs); the broker client claims one and
          # binds it, passing its bridge IP for the inbound DNAT.
          broker = "http://${config.infra.clients.container-vpn-gw.ip}:${toString config.infra.clients.container-vpn-gw.ports.control}";
          vpnPool = qbtVpnPool;
          # Re-lock the MAM session to the current VPN exit IP on failover (ASN-locked session;
          # allowlist covers all pool exit ASNs). Seed cookie in sops (mam-vpn-id).
          mam.enable = true;
          # eth0 stays 1500 (LAN). Torrent traffic egresses the vpnN veths, which
          # qbt-routing caps to 1420 (tunnel path-MTU) so outbound uTP isn't blackholed.
          watchDir = "/mnt/media/watch";
          inherit categories;
          watchIntervalSec = 10;
          tagSyncIntervalSec = 60;
          # %N name, %L category, %Z size-bytes, %I infohash-v1. No quotes: qbt splits
          # the program string into argv FIRST, then substitutes each %-token in place,
          # so %N stays a single arg even with spaces. Quoting instead gets mangled by
          # qbt's conf writer (it ate the quotes and collapsed the spaces → one arg).
          onTorrentFinished = "${onFinished} %N %L %Z %I";
          onTorrentAdded = "${onAdded} %N %L %Z";

          qbitManage = import ./_qbit-manage.nix {
            inherit lib categories;
            webuiPort = config.infra.clients.container-qbittorrent.ports.webui;
          };

          qbittorrentExporter = {
            enable = true;
            port = config.infra.clients.container-qbittorrent.ports.exporter;
            highCardinality = true;
          };

          bitTorrentConfig = import ./_bittorrent.nix;

          preferences = {
            webUIAddress = "*";
            webUILocalHostAuth = false;
            authSubnetWhitelistEnabled = true;
            authSubnetWhitelist = [
              (cidr config.infra.vlans.apps)
              (cidr config.infra.vlans.media)
              # HA (iot VLAN) needs unauth qbt add for autobrr 👍-approved grabs.
              # /32 only — not the whole iot subnet (untrusted devices).
              "${config.infra.vlans.iot.staticAddresses.home-assistant}/32"
            ];
          };
          bindMounts = {
            "/var/lib/qbittorrent" = {
              hostPath = "/var/lib/appdata/qbittorrent";
              isReadOnly = false;
            };
            "/mnt/media" = {
              hostPath = "/mnt/media";
              isReadOnly = false;
            };
            "/var/tmp/qbittorrent" = {
              hostPath = "/var/tmp/qbittorrent";
              isReadOnly = false;
            };
            "/var/lib/qbittorrent/qBittorrent/config/categories.json" = {
              hostPath = "${categoriesJson}";
              isReadOnly = true;
            };
            "/run/secrets/cross-seed-apikey" = {
              hostPath = config.sops.secrets.cross-seed-apikey.path;
              isReadOnly = true;
            };
            "/run/secrets/mam-vpn-id" = {
              hostPath = config.sops.secrets.mam-vpn-id.path;
              isReadOnly = true;
            };
          };
          nameservers = [
            "1.1.1.1"
            "8.8.8.8"
          ];
          logging = {
            enable = true;
            lokiAddress = "http://${config.infra.clients.container-loki.ip}:${toString config.infra.clients.container-loki.ports.default}";
            prometheusAddress = "http://${config.infra.clients.container-prometheus.ip}:${toString config.infra.clients.container-prometheus.ports.default}/api/v1/write";
            tempoAddress = "http://${config.infra.clients.container-tempo.ip}:${toString config.infra.clients.container-tempo.ports.otlp-http}/v1/traces";
            inherit
              (
                (import ./_exporter.nix {
                  exporterPort = config.infra.clients.container-qbittorrent.ports.exporter;
                  webuiPort = config.infra.clients.container-qbittorrent.ports.webui;
                  inherit pkgs lib;
                })
              )
              prometheusInputs
              ;
            # qbt AutoRun hooks POST to the local Vector sidecar (loopback); republished
            # verbatim to MQTT so HA consumes via an mqtt trigger instead of a webhook.
            extraSettings =
              lib.recursiveUpdate
                (import ./_exporter.nix {
                  exporterPort = config.infra.clients.container-qbittorrent.ports.exporter;
                  webuiPort = config.infra.clients.container-qbittorrent.ports.webui;
                  inherit pkgs lib;
                }).extraSettings
                {
                  sources.webhook_in = {
                    type = "http_server";
                    address = "127.0.0.1:9001";
                    method = "POST";
                    path = "/";
                    decoding.codec = "bytes"; # keep the AutoRun JSON body verbatim → .message
                    framing.method = "bytes"; # one event per request body, not per line
                  };
                  sinks.webhook_mqtt = {
                    type = "mqtt";
                    inputs = [ "webhook_in" ];
                    host = config.infra.clients.container-mosquitto.ip;
                    port = config.infra.clients.container-mosquitto.ports.default;
                    client_id = "vector-qbittorrent"; # unique per container → no broker client-id collision
                    topic = "homelab/notify/qbittorrent";
                    quality_of_service = "atleastonce";
                    retain = false;
                    encoding.codec = "text"; # emit .message raw (the AutoRun JSON)
                  };
                };
          };
        };

        systemd.tmpfiles.rules = [
          "d /var/lib/appdata/qbittorrent 0770 ${toString cfg.uid} ${toString cfg.gid} - -"
          "d /var/lib/appdata/qbittorrent/qBittorrent 0770 ${toString cfg.uid} ${toString cfg.gid} - -"
          "d /var/lib/appdata/qbittorrent/qBittorrent/config 0770 ${toString cfg.uid} ${toString cfg.gid} - -"
          "f /var/lib/appdata/qbittorrent/qBittorrent/config/categories.json 0660 ${toString cfg.uid} ${toString cfg.gid} - -"
          # incomplete downloads — local, so in-flight random writes don't hit NFS
          "d /var/tmp/qbittorrent 0770 ${toString cfg.uid} ${toString cfg.gid} - -"
          "x /var/tmp/qbittorrent" # exempt from the 30d /var/tmp age reaper
          "h /var/tmp/qbittorrent - - - - +C" # nodatacow, inherited by new files
        ];

        systemd.services."container@qbittorrent" = {
          before = [ "container-wave-core.target" ];
          # Resource isolation. qbt is a maxed-out seedbox (128 async-IO threads, MMap
          # disk IO over NFS-backed /mnt/media). Under heavy seeding its mmap/page-cache
          # growth drove host-wide memory reclaim + swap, starving co-tenant services
          # (notably dnsmasq) of scheduling → intermittent DNS timeouts. MemoryHigh makes
          # the kernel throttle and reclaim qbt's OWN pages before it pressures the host;
          # MemoryMax is the hard ceiling (peak observed ~52G, box has 91G); low CPUWeight
          # yields CPU to system services under contention. NOTE: IOWeight is deliberately
          # NOT used — NFS IO is not accounted by the block-io cgroup controller.
          serviceConfig = {
            MemoryHigh = "56G";
            MemoryMax = "64G";
            CPUWeight = 30;
          };
          after = [
            "remote-fs.target"
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
            "container-wave-foundation.target"
          ];
          # br-vpnN have no host IP → nothing pulls their netdevs in; require them so the
          # bridges exist before the container enslaves its veths.
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
          # /mnt/media is an x-systemd.automount (noauto) → NOT in remote-fs.target, so the
          # above `after` doesn't guarantee it's mounted. Force the real mount before the
          # container starts (RequiresMountsFor pulls in + orders after mnt-media.mount);
          # otherwise nspawn bind-mounts an empty autofs trigger dir and qbt bails at boot.
          unitConfig.RequiresMountsFor = "/mnt/media";
        };

        # Enslave qbt's per-tunnel veths (vpnN → br-vpnN) the INSTANT nspawn creates
        # them, on the udev `add` event — before the container's init runs. nixos-container
        # only does this in its ExecStartPost (`ip link set vpn1 master br-vpn1 up`), which
        # runs AFTER the container signals ready. That's too late: the in-container startup
        # (conf_patch NAT-PMP) needs the tunnel reachable, and blocking on it prevents ready
        # → ExecStartPost never runs → veth never bridged → deadlock. qbt-routing can't fix
        # it either (it's in the container netns; br-vpnN is a host bridge). Doing the same
        # enslave early makes the p2p link live at startup so natpmp works.
        services.udev.extraRules = ''
          ACTION=="add", SUBSYSTEM=="net", KERNEL=="vpn[0-9]*", RUN+="${pkgs.iproute2}/bin/ip link set %k master br-%k up"
        '';
      };
  };
}
