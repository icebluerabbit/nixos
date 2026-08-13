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
              # The search API sends a numeric mediatype id where the IRC announce
              # sends a name; this is the join between them. Ids confirmed by
              # correlating a live search page against the post-2026-08-11 announce
              # stream. Ids 3 and 4 exist (Musicology sheet music, Radio) but no
              # torrent of either has been announced since the format change, so
              # nothing ties them to a name — left out deliberately. They arrive as
              # "3"/"4", match no list below, and show up as such in the metrics.
              mediaTypeNames = {
                "1" = "Audiobook";
                "2" = "Ebook";
                "5" = "Manga";
                "6" = "Comic Book / Graphic Novel";
                "7" = "Periodical Ebook";
              };
            };
            # Reactive delivery target for armed grabs.
            client = {
              url = "http://${config.infra.clients.container-qbittorrent.ip}:${toString config.infra.clients.container-qbittorrent.ports.webui}";
              # Keyed by MAM media type. Manga and Comic Book were the "Ebooks -
              # Comics/Graphic novels" subcategory before MAM promoted them to media
              # types of their own, so they keep landing in the books category.
              categories = {
                "Ebook" = "books";
                "Periodical Ebook" = "books";
                "Manga" = "books";
                "Comic Book / Graphic Novel" = "books";
                "Audiobook" = "audiobooks";
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
              # MAM's own media-type vocabulary; matched case-insensitively. Manga and
              # Comic Book / Graphic Novel are listed explicitly because they used to
              # arrive as the "Ebooks - Comics/Graphic novels" category (61 of the
              # first 1236 announces) and would otherwise be dropped outright.
              mediaTypes = [
                "Ebook"
                "Periodical Ebook"
                "Manga"
                "Comic Book / Graphic Novel"
                "Audiobook"
              ];
              # SciFi / Fantasy / Horror. Categories are multi-valued as of 2026-08-11
              # and a release passes on ANY match, so entries lost their "Ebooks - " /
              # "Audiobooks - " prefix — that split is the media type above now.
              #
              # MAM also RE-WORDED the taxonomy in that change, so the de-prefixed old
              # names were dead strings: Crime/Thriller became Crime + Thriller/Suspense,
              # General Fiction became Contemporary Fiction, Historical Fiction became
              # Historical, and Comics/Graphic novels stopped being a category at all
              # (comics are a media type above). Every entry below has been observed in a
              # live post-cutover announce; History, True Crime and Young Adult were
              # dropped because they have not been — re-add only once seen in
              # announces_by_category_total.
              categoriesAllow = [
                "Action/Adventure"
                "Contemporary Fiction"
                "Crime"
                "Fantasy"
                "Historical"
                "Horror"
                "LitRPG"
                "Literary Classics"
                "Mystery"
                "Paranormal"
                "Progression Fantasy"
                "Science Fiction"
                "Superheroes"
                "Thriller/Suspense"
                "Urban Fantasy"
              ];
              categoriesDeny = [ ];
              # Filetypes are multi-valued too ("azw3, epub") and match on ANY listed
              # format. Exact whole-string matching used to reject 7 of the first 1236
              # announces that carried an allowed format alongside another.
              filetypesAllow = [
                "m4b"
                "mp3"
                "epub"
                # Comics/manga. 80 of the 95 filetype rejects in the week to 2026-08-13
                # arrive as media type Ebook rather than Comic Book / Graphic Novel, so
                # the mediaTypes list above never gated them — this rule did. Worth
                # 7.3/day (34 of 51 VIP-freeleech), 25 GiB/week. Satisfy is seedtime-only,
                # so a comic occupies an unsat slot exactly as long as an ebook and pays
                # the same satisfied-torrent points: size is free, disk is not scarce.
                "cbr"
                "cbz"
              ];
              filetypesDeny = [ ];
              # Fiction only. Categories alone cannot express this — of the 22 non-fiction
              # announces in the first 12h after the format change, 3 carried a category
              # from the allowlist above (Crime; Historical; Paranormal) and would have
              # been accepted. Torznab search results carry no flag and are unaffected.
              fictionTypes = [ "Fiction" ];
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
              proactiveMediaTypes = [ "Audiobook" ];
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
