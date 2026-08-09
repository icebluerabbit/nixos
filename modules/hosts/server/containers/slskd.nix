{ den, ... }:
{
  den.aspects.hosts._.server._.containers._.slskd = {
    aspect-name = "hosts.server.containers.slskd";
    includes = [
      den.aspects.containers._.slskd
      den.aspects.containers._.vpn-gw # for the tunnel pool
      den.aspects.infra._.clients._.container-slskd
      den.aspects.infra._.clients._.container-vpn-gw # broker addr
      den.aspects.infra._.clients._.container-dns
      den.aspects.infra._.clients._.container-mosquitto
      den.aspects.infra._.networks._.download
      den.aspects.hosts._.server._.security
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
        cfg = config.containerConfigs.slskd;
        # slskd's own veth IP on each tunnel bridge (.3; vpn-gw=.1, qbt=.2). slskd shares the
        # br-vpnN pool with qbt, so its host veths must NOT collide with qbt's vpn1..5 — hence
        # the sl-prefixed names. Everything else derives from the vpn-gw tunnel pool.
        slskdVpnIps = {
          wg1 = "192.168.1.3";
          wg2 = "192.168.2.3";
          wg3 = "192.168.3.3";
          wg4 = "192.168.4.3";
          wg5 = "192.168.5.3";
          wg6 = "192.168.6.3";
          wg7 = "192.168.7.3";
          wg8 = "192.168.8.3";
          wg9 = "192.168.9.3";
          wg10 = "192.168.10.3";
        };
        slskdVpnPool = map (t: {
          tunnel = t.name;
          veth = "sl" + lib.removePrefix "br-" t.bridge; # br-vpn1 -> slvpn1
          ip = slskdVpnIps.${t.name};
          gateway = t.ip; # vpn-gw side of the veth
          inherit (t) bridge;
        }) config.containerConfigs."vpn-gw".tunnels;
      in
      {
        containerConfigs.slskd = {
          lldpd.enable = true;
          hostBridge = "br-download";
          macAddress = config.infra.clients.container-slskd.mac;
          localAddress = "${config.infra.clients.container-slskd.ip}/${toString config.infra.vlans.download.subnetPrefix}";
          gateway = config.infra.vlans.download.staticAddresses.gateway; # LAN carve-out next-hop
          port = config.infra.clients.container-slskd.ports.web;
          broker = "http://${config.infra.clients.container-vpn-gw.ip}:${toString config.infra.clients.container-vpn-gw.ports.control}";
          vpnPool = slskdVpnPool;
          envFile = "/run/secrets/slskd-env"; # in-container path (bind-mounted below)
          # Share the organized library AND the raw completed downloads. The downloadDir is
          # itself a share here — the nixpkgs slskd module puts every share in ReadOnlyPaths,
          # which would collide with the downloadDir's ReadWritePath and make slskd refuse to
          # start ("not writeable"). The aspect strips the downloadDir back out of ReadOnlyPaths
          # (lib.mkForce) so it stays read-write and still shareable.
          shares = [
            "/mnt/media/library/music"
            "/mnt/media/complete/soulseek"
          ]; # what slskd seeds to Soulseek

          # performance/policy tuning — server: 91G RAM / 32-thread EPYC / 2Gbe WAN. Aggressive
          # for fastest transfers + most seeding; the real ceiling is the Proton tunnel + peers,
          # not the box. Knobs not set here keep the aspect's conservative defaults (inactivity
          # timeout, up/down speed + download slots unlimited, leecher priority).
          childLimit = 256; # big branch parent → the shared library reaches more searchers
          connectTimeout = 15000; # tolerate slow-to-connect peers (VPN adds latency)
          transferTimeout = 60000;
          readBuffer = 65536;
          writeBuffer = 65536;
          transferBuffer = 16777216; # 16 MiB — saturate fast peers/tunnel; trivial on 91G RAM
          writeQueue = 100; # smoother sustained writes
          shareCacheMode = "memory"; # RAM share index → fast searches
          shareCacheWorkers = 32; # = core count
          uploadSlots = 100; # serve many peers at once = most seeding
          searchConcurrency = 100; # answer more incoming searches
          searchCircuitBreaker = 2000; # raise with concurrency or searches get shed
          searchResponseFileLimit = 4096;

          nameservers = [
            config.infra.clients.container-dns.ip
            "1.1.1.1"
          ];
          bindMounts = {
            "/var/lib/slskd" = {
              hostPath = "/var/lib/appdata/slskd";
              isReadOnly = false;
            };
            "/mnt/media" = {
              hostPath = "/mnt/media";
              isReadOnly = false;
            };
            "/run/secrets/slskd-env" = {
              hostPath = config.sops.templates."slskd-env".path;
              isReadOnly = true;
            };
          };
          logging = {
            enable = true;
            lokiAddress = "http://${config.infra.clients.container-loki.ip}:${toString config.infra.clients.container-loki.ports.default}";
            prometheusAddress = "http://${config.infra.clients.container-prometheus.ip}:${toString config.infra.clients.container-prometheus.ports.default}/api/v1/write";
            tempoAddress = "http://${config.infra.clients.container-tempo.ip}:${toString config.infra.clients.container-tempo.ports.otlp-http}/v1/traces";
            prometheusInputs = [ "slskd_metrics" ];
            extraSettings = {
              sources.slskd_metrics = {
                type = "prometheus_scrape";
                endpoints = [
                  "http://localhost:${toString config.infra.clients.container-slskd.ports.web}/metrics"
                ];
                scrape_interval_secs = 30;
              };
              # Per-file download completed/failed → MQTT homelab/notify/soulseek → HA → #soulseek.
              # slskd has no webhook; parse its journal (one clean line per transfer).
              transforms.slskd_notify = {
                type = "remap";
                inputs = [ "journal" ];
                source = ''
                  msg = to_string(.message) ?? ""
                  ev = null
                  file = null
                  user = null
                  reason = null
                  m = parse_regex(msg, r'Download of (?P<file>.+?) from user (?P<user>\S+) completed successfully') ?? null
                  if !is_null(m) {
                      ev = "download_completed"
                      file = m.file
                      user = m.user
                  }
                  if is_null(ev) {
                      m = parse_regex(msg, r'Download of (?P<file>.+?) from user (?P<user>\S+) failed: (?P<reason>[^:]+)') ?? null
                      if !is_null(m) {
                          ev = "download_failed"
                          file = m.file
                          user = m.user
                          reason = m.reason
                      }
                  }
                  if is_null(ev) {
                      abort
                  }
                  .message = encode_json({"source": "slskd", "event": ev, "file": file, "user": user, "reason": reason})
                '';
              };
              sinks.slskd_notify_mqtt = {
                type = "mqtt";
                inputs = [ "slskd_notify" ];
                host = config.infra.clients.container-mosquitto.ip;
                port = config.infra.clients.container-mosquitto.ports.default;
                client_id = "vector-slskd"; # unique per container → no broker client-id collision
                topic = "homelab/notify/soulseek";
                quality_of_service = "atleastonce";
                retain = false;
                encoding.codec = "text"; # emit .message (the single-event JSON)
              };
            };
          };
        };

        # Enslave slskd's veths to their br-vpnN. The name (slvpnN) doesn't derive the bridge,
        # so map each explicitly rather than qbt's `master br-%k` shortcut.
        services.udev.extraRules = lib.concatMapStringsSep "\n" (
          v:
          ''ACTION=="add", SUBSYSTEM=="net", KERNEL=="${v.veth}", RUN+="${pkgs.iproute2}/bin/ip link set %k master ${v.bridge} up"''
        ) slskdVpnPool;

        systemd = {
          tmpfiles.rules = [
            "d /var/lib/appdata/slskd 0770 ${toString cfg.uid} ${toString cfg.gid} - -"
            "Z /var/lib/appdata/slskd 0770 ${toString cfg.uid} ${toString cfg.gid} - -"
            # incomplete downloads — local (under the state dir), so in-flight random writes don't
            # hit NFS. Under /var/lib (not /var/tmp) so slskd's PrivateTmp doesn't shadow it.
            "d /var/lib/appdata/slskd/incomplete 0770 ${toString cfg.uid} ${toString cfg.gid} - -"
            "h /var/lib/appdata/slskd/incomplete - - - - +C" # nodatacow, inherited by new files
          ];

          services = {
            "container@slskd" = {
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
              ];
              # br-vpnN carry no host IP → nothing else pulls their netdevs in; require them so the
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
              # /mnt/media is an x-systemd.automount (noauto) → force the real mount before start.
              unitConfig.RequiresMountsFor = "/mnt/media";
            };
          };
        };
      };
  };
}
