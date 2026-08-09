{ den, ... }:
{
  den.aspects.hosts._.server._.containers._.mousetrap = {
    aspect-name = "hosts.server.containers.mousetrap";
    includes = [
      den.aspects.containers._.mousetrap
      den.aspects.infra._.clients._.container-mousetrap
      den.aspects.infra._.clients._.container-qbittorrent
      den.aspects.infra._.clients._.container-dns
      den.aspects.infra._.networks._.media
      den.aspects.hosts._.server._.security
      den.aspects.hosts._.server._.networking._.vlans._.media
      den.aspects.infra._.clients._.container-loki
      den.aspects.infra._.clients._.container-prometheus
      den.aspects.infra._.clients._.container-tempo
      den.aspects.infra._.clients._.container-mosquitto
    ];

    nixos =
      { config, ... }:
      let
        cfg = config.containerConfigs.mousetrap;
      in
      {
        containerConfigs.mousetrap = {
          lldpd.enable = true;
          hostBridge = "br-media";
          macAddress = config.infra.clients.container-mousetrap.mac;
          localAddress = "${config.infra.clients.container-mousetrap.ip}/${toString config.infra.vlans.media.subnetPrefix}";
          gateway = config.infra.vlans.media.staticAddresses.gateway;
          port = config.infra.clients.container-mousetrap.ports.default;
          # MAM grab authority — every value explicit (even defaults) so the whole policy
          # lives here. Domains mirror the binary flags; refresh drives the /refresh|/shop
          # timers (own rotating session).
          settings = {
            enable = true;
            mam = {
              apiBase = "https://www.myanonamouse.net";
              seedSecretPath = "/run/secrets/mam-api-id";
            };
            # Reactive delivery target for armed grabs.
            client = {
              url = "http://${config.infra.clients.container-qbittorrent.ip}:${toString config.infra.clients.container-qbittorrent.ports.webui}";
              categories = {
                ebook = "books";
                audiobook = "audiobooks";
              };
              addPaused = false;
            };
            # Collector cadence (params baked into the binary; timers just curl the endpoints).
            refresh = {
              account = {
                enable = true;
                intervalSec = 60;
              };
              freeleech = {
                enable = true;
                intervalSec = 1800;
              };
              bonus = {
                enable = true;
                intervalSec = 3600;
              };
              shop = {
                enable = true;
                intervalSec = 3600;
              };
            };
            shop = {
              dryRun = true; # log "would buy", spend nothing until armed
              reserved = 1000;
              vip = {
                enable = true;
                cost = 5000;
                reserveMultiple = 2;
                buyableClasses = [ ]; # add your class (Power User+) to arm VIP buys
              };
              upload = {
                enable = true;
                chunkGiB = 50;
                pointsPerGiB = 500;
              };
            };
            # Torznab indexer for the *arr ecosystem — links minted by search route back
            # here so grabs still run the full policy. No auth on the trusted media VLAN.
            torznab = {
              apiKey = "";
              baseUrl = "http://${config.infra.clients.container-mousetrap.ip}:${toString config.infra.clients.container-mousetrap.ports.default}";
              linkTtlSec = 3600;
            };
            # Notify → local Vector sidecar (loopback :9001) → MQTT homelab/notify/mousetrap
            # → HA mqtt trigger → #myanonamouse. Same chain as the *arr containers. Body is
            # {"type","payload"}; HA branches on .type and renders per-event cards.
            webhook = {
              url = "http://127.0.0.1:9001/";
              headers."Content-Type" = "application/json";
              events = [
                "announce"
                "grab"
                "reject"
                "hold"
                "enqueue"
                "bonus_received"
                "spend"
              ];
              template = ''{"type":"{{ .Type }}","payload":{{ toRawJson .Payload }}}'';
            };
            # Layer-1 accept criteria — logs mousetrap_would_accept_total{decision,reason}
            # per candidate. Start broad (books + audiobooks, any category/size) so real
            # traffic fills the tuning dashboard; tighten from what you see. Every value
            # explicit even at default. Empty list / 0 / "" = rule off.
            accept = {
              kinds = [
                "ebook"
                "audiobook"
              ];
              # SciFi / Fantasy / Horror, both mains. Exact strings verified against the
              # search catnames (Urban Fantasy is a separate category, not included).
              categoriesAllow = [
                "Audiobooks - Science Fiction"
                "Audiobooks - Fantasy"
                "Audiobooks - Horror"
                "Audiobooks - Young Adult"
                "Audiobooks - True Crime" # True Crime is audiobook-only on MAM
                "Audiobooks - Urban Fantasy"
                "Audiobooks - Literary Classics"
                "Audiobooks - History"
                "Audiobooks - General Fiction"
                "Audiobooks - Historical Fiction"
                "Audiobooks - Action/Adventure"
                "Audiobooks - Crime/Thriller"
                "Audiobooks - Mystery"
                "Ebooks - Science Fiction"
                "Ebooks - Fantasy"
                "Ebooks - Horror"
                "Ebooks - Comics/Graphic novels" # ebooks only
                "Ebooks - Young Adult"
                "Ebooks - Urban Fantasy"
                "Ebooks - Literary Classics"
                "Ebooks - History"
                "Ebooks - General Fiction"
                "Ebooks - Historical Fiction"
                "Ebooks - Action/Adventure"
                "Ebooks - Crime/Thriller"
                "Ebooks - Mystery"
              ];
              categoriesDeny = [ ];
              # exact match on the announce filetype tag (single-format in practice)
              filetypesAllow = [
                "m4b"
                "mp3"
                "epub"
              ];
              filetypesDeny = [ ];
              languagesAllow = [ "english" ]; # substring, case-insensitive; empty-language announces reject
              sizeMinBytes = 0; # no floor
              sizeMaxBytes = 0; # no ceiling
              freeleechOnly = false;
              vipOnly = false; # raw release vip flag; VIP-freeleech-for-us is a Layer-2 check
              excludeVip = false;
              nameIncludeRegex = "";
              nameExcludeRegex = "";
            };
            # Layer-2 grab gate — account-state check on Layer-1 survivors. dryRun=true keeps it
            # decision-only (logs would_grab, adds nothing); flip dryRun=false to arm real grabs.
            grab = {
              unsatMargin = 2; # hold 3 below unsat_limit (effective = real unsat + in-flight count)
              inFlightWindowSec = 1800; # 30min — covers MAM unsat.count + downloaded_bytes lag; expiry reconciles both
              bufferMinBytes = 2147483648; # 2 GiB — hold non-FL grabs when (buffer − in-flight downloads) drops below
              dryRun = false; # ARM SWITCH — true = add nothing; false = fetch + deliver to client
            };
            # Personal Freeleech wedge strategy (usetoken=FL). Wedged grab = freeleech (no
            # ratio/buffer cost), spends 1 of the 10-wedge stock. Optimistic in-flight count.
            wedge = {
              enable = true;
              reserve = 2; # keep at least 2 wedges in reserve
              # B — proactive: always wedge big audiobooks (400 MB ≈ median; ratio-first)
              proactiveKinds = [ "audiobook" ];
              proactiveMinSizeBytes = 400000000; # 400 MB
              # A — rescue: wedge a would-be buffer hold on anything ≥ 100 MB
              rescueEnable = true;
              rescueMinSizeBytes = 100000000; # 100 MB
            };
          };
          bindMounts = {
            "/var/lib/mousetrap" = {
              hostPath = "/var/lib/appdata/mousetrap";
              isReadOnly = false;
            };
            "/run/secrets/mam-api-id" = {
              hostPath = config.sops.secrets.mam-api-id.path;
              isReadOnly = true;
            };
          };
          nameservers = [
            config.infra.clients.container-dns.ip
            "1.1.1.1"
          ];
          logging = {
            enable = true;
            lokiAddress = "http://${config.infra.clients.container-loki.ip}:${toString config.infra.clients.container-loki.ports.default}";
            prometheusAddress = "http://${config.infra.clients.container-prometheus.ip}:${toString config.infra.clients.container-prometheus.ports.default}/api/v1/write";
            tempoAddress = "http://${config.infra.clients.container-tempo.ip}:${toString config.infra.clients.container-tempo.ports.otlp-http}/v1/traces";
            # Vector scrapes mousetrap's /metrics (prometheus_client) and remote-writes it.
            prometheusInputs = [ "mousetrap_metrics" ];
            extraSettings = {
              sources.mousetrap_metrics = {
                type = "prometheus_scrape";
                endpoints = [
                  "http://127.0.0.1:${toString config.infra.clients.container-mousetrap.ports.default}/metrics"
                ];
                scrape_interval_secs = 30;
              };
              # mousetrap's webhook POSTs here on loopback; republish verbatim to MQTT so
              # HA consumes via an mqtt trigger (no notify-time dependency on HA).
              sources.webhook_in = {
                type = "http_server";
                address = "127.0.0.1:9001";
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
                client_id = "vector-mousetrap"; # unique per container
                topic = "homelab/notify/mousetrap";
                quality_of_service = "atleastonce";
                retain = false;
                encoding.codec = "text"; # emit .message raw (the mousetrap JSON)
              };
            };
          };
        };

        systemd.tmpfiles.rules = [
          "d /var/lib/appdata/mousetrap 0770 ${toString cfg.uid} ${toString cfg.gid} - -"
          "Z /var/lib/appdata/mousetrap 0770 ${toString cfg.uid} ${toString cfg.gid} - -"
        ];
      };
  };
}
