# bindery — host configuration override for bindery container
{ den, ... }:
{
  den.aspects.hosts._.server._.containers._.bindery = {
    aspect-name = "hosts.server.containers.bindery";
    includes = [
      den.aspects.containers._.bindery
      den.aspects.infra._.clients._.container-bindery
      den.aspects.infra._.clients._.container-qbittorrent
      den.aspects.infra._.clients._.container-mousetrap
      den.aspects.infra._.clients._.container-mosquitto
      den.aspects.infra._.clients._.container-caddy
      den.aspects.infra._.clients._.container-dns
      den.aspects.infra._.networks._.media
      den.aspects.infra._.networks._.trust
      den.aspects.hosts._.server._.security
      den.aspects.hosts._.server._.networking._.vlans._.media
      den.aspects.infra._.clients._.container-loki
      den.aspects.infra._.clients._.container-prometheus
      den.aspects.infra._.clients._.container-tempo
    ];

    nixos =
      { config, ... }:
      let
        cfg = config.containerConfigs.bindery;
      in
      {
        containerConfigs.bindery = {
          lldpd.enable = true;
          hostBridge = "br-media";
          macAddress = config.infra.clients.container-bindery.mac;
          localAddress = "${config.infra.clients.container-bindery.ip}/${toString config.infra.vlans.media.subnetPrefix}";
          gateway = config.infra.vlans.media.staticAddresses.gateway;
          port = config.infra.clients.container-bindery.ports.default;

          # Both entries are required, and the second is not a mistake.
          #
          # Bindery's real-IP middleware runs before its proxy-auth check. Seeing
          # a request from a trusted Caddy, it walks X-Forwarded-For and rewrites
          # RemoteAddr to the first untrusted hop — the browser. The proxy-auth
          # check then reads that same RemoteAddr and tests it against this list,
          # so trusting only Caddy makes it reject every request with
          # "identity header from untrusted source — rejecting, peer=<client>".
          # The client range therefore has to be here too.
          #
          # The exposure is bounded: Remote-User is only honoured on requests
          # that already carried a valid Authelia session through Caddy, and the
          # trust VLAN is this household's own machines.
          trustedProxies = [
            "${config.infra.clients.container-caddy.ip}/32"
            "${config.infra.vlans.trust.subnet}/${toString config.infra.vlans.trust.subnetPrefix}"
          ];

          # complete/ is the torrent data qBittorrent keeps seeding; library/ holds
          # the hardlinks Bindery creates. Both sit under the same NFS export, which
          # is what makes hardlinking possible at all — verified same st_dev.
          downloadDir = "/mnt/media/complete";
          libraryDir = "/mnt/media/library/books";
          audiobookDir = "/mnt/media/library/audiobooks";

          configuratarr = {
            enable = true;
            waitForHealthy = true;
            settings.bindery = {
              type = "bindery-v1";
              url = "http://localhost:${toString config.infra.clients.container-bindery.ports.default}";
              api_key = "\${env.BINDERY_API_KEY}";
              root_folders = [
                { path = "/mnt/media/library/books"; }
                { path = "/mnt/media/library/audiobooks"; }
              ];
              download_clients = [
                (import ./download-clients/_qbittorrent.nix { inherit config; })
              ];
              # Disabled: Bindery's own auto-grab against MAM is off for now.
              # NOTE: configuratarr runs without --prune, so removing this entry
              # does NOT delete the indexer row already in bindery.db — set
              # `enabled = false` in _mousetrap.nix if the live indexer must stop.
              # indexers = [
              #   (import ./indexers/_mousetrap.nix { inherit config; })
              # ];
              # Notify → local Vector sidecar (loopback :9001) → MQTT
              # homelab/notify/bindery → HA mqtt trigger → #bindery. Same chain as
              # the *arr containers. Bindery has exactly one delivery
              # implementation (an outbound JSON POST), so notification_type is a
              # label only — dispatch is purely on url/topic/method.
              notifications = [
                {
                  name = "Vector";
                  notification_type = "webhook";
                  # NOT loopback. Bindery validates notification URLs against its
                  # strict SSRF policy, which rejects 127.0.0.1 outright
                  # ("url not allowed: points to loopback address") — unlike
                  # indexers and download clients, notifications never get
                  # PolicyLANLoopback, and there is no production escape hatch.
                  # So the sidecar listens on this container's own VLAN address
                  # instead. Still not reachable from the VLAN: _base.nix opens
                  # only cfg.port in the container firewall, so 9001 is blocked
                  # from outside while traffic to our own IP routes over lo.
                  # Requires BINDERY_NOTIFICATIONS_ALLOW_PRIVATE (see the
                  # container aspect) because the address is RFC1918.
                  url = "http://${config.infra.clients.container-bindery.ip}:9001/";
                  method = "POST";
                  headers = builtins.toJSON { "Content-Type" = "application/json"; };
                  enabled = true;
                  on_grab = true;
                  on_import = true;
                  on_upgrade = true;
                  on_failure = true;
                  on_health = true;
                }
              ];
              # Singleton is presence-masked: only these keys are managed, the
              # rest stay live. import_mode is the seeding guarantee — library
              # files are hardlinks to torrent data qBittorrent is still seeding
              # in /mnt/media/complete, and both trees are one NFS export. The
              # default `auto` silently degrades to copy; `move` relocates the
              # file out from under the torrent and breaks seeding outright.
              # Same rule Radarr enforces via media_management.copy_using_hardlinks.
              setting = {
                import_mode = "hardlink";
                hardcover_api_token = "\${env.HARDCOVER_API_KEY}";
              };
              # Trust Caddy's Authelia-populated Remote-User header instead of
              # showing Bindery's own login form. One login (Authelia), no second
              # prompt. Note this makes the header authoritative, so anyone able
              # to reach 10.10.120.23:8787 directly — bypassing Caddy — could set
              # it themselves; that is bounded by the media VLAN.
              auth_mode = {
                mode = "proxy";
              };
            };
          };

          bindMounts = {
            "/var/lib/bindery" = {
              hostPath = "/var/lib/appdata/bindery";
              isReadOnly = false;
            };
            "/mnt/media" = {
              hostPath = "/mnt/media";
              isReadOnly = false;
            };
          };
          envFile = config.sops.templates."bindery-env".path;
          nameservers = [
            config.infra.clients.container-dns.ip
            "1.1.1.1"
          ];
          logging = {
            enable = true;
            lokiAddress = "http://${config.infra.clients.container-loki.ip}:${toString config.infra.clients.container-loki.ports.default}";
            prometheusAddress = "http://${config.infra.clients.container-prometheus.ip}:${toString config.infra.clients.container-prometheus.ports.default}/api/v1/write";
            tempoAddress = "http://${config.infra.clients.container-tempo.ip}:${toString config.infra.clients.container-tempo.ports.otlp-http}/v1/traces";
            prometheusInputs = [ "bindery_metrics" ];
            extraSettings = {
              # Bindery mounts /metrics at its router root with no auth, no CSRF
              # and no API key — upstream's deliberate choice, since Prometheus
              # scrapes carry no session cookie. Loopback-only from the sidecar.
              sources.bindery_metrics = {
                type = "prometheus_scrape";
                endpoints = [
                  "http://127.0.0.1:${toString config.infra.clients.container-bindery.ports.default}/metrics"
                ];
                scrape_interval_secs = 30;
              };
              # Bindery's webhook notification POSTs here on loopback; republish
              # verbatim to MQTT so HA consumes via an mqtt trigger (no
              # notify-time dependency on HA).
              sources.webhook_in = {
                type = "http_server";
                # Bindery refuses to POST to loopback (SSRF policy), so bind the
                # container's VLAN address. The container firewall only opens
                # cfg.port, so this stays unreachable from the VLAN itself.
                address = "${config.infra.clients.container-bindery.ip}:9001";
                method = "POST";
                path = "/";
                decoding.codec = "bytes"; # keep the JSON body verbatim → .message
                framing.method = "bytes"; # one event per request body
              };
              sinks.webhook_mqtt = {
                type = "mqtt";
                inputs = [ "webhook_in" ];
                host = config.infra.clients.container-mosquitto.ip;
                port = config.infra.clients.container-mosquitto.ports.default;
                client_id = "vector-bindery"; # unique per container
                topic = "homelab/notify/bindery";
                quality_of_service = "atleastonce";
                retain = false;
                encoding.codec = "text"; # emit .message raw (the bindery JSON)
              };
            };
          };
        };

        systemd.tmpfiles.rules = [
          "d /var/lib/appdata/bindery 0770 ${toString cfg.uid} ${toString cfg.gid} - -"
          "Z /var/lib/appdata/bindery 0770 ${toString cfg.uid} ${toString cfg.gid} - -"
        ];
      };
  };
}
