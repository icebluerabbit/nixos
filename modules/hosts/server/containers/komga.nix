# komga — host configuration override for komga container
{ den, ... }:
{
  den.aspects.hosts._.server._.containers._.komga = {
    aspect-name = "hosts.server.containers.komga";
    includes = [
      den.aspects.containers._.komga
      den.aspects.infra._.clients._.container-komga
      den.aspects.infra._.clients._.container-dns
      den.aspects.infra._.networks._.media
      den.aspects.hosts._.server._.security
      den.aspects.hosts._.server._.networking._.vlans._.media
      den.aspects.infra._.clients._.container-loki
      den.aspects.infra._.clients._.container-prometheus
      den.aspects.infra._.clients._.container-tempo
    ];

    nixos =
      { config, ... }:
      let
        cfg = config.containerConfigs.komga;
      in
      {
        containerConfigs.komga = {
          lldpd.enable = true;
          hostBridge = "br-media";
          macAddress = config.infra.clients.container-komga.mac;
          localAddress = "${config.infra.clients.container-komga.ip}/${toString config.infra.vlans.media.subnetPrefix}";
          gateway = config.infra.vlans.media.staticAddresses.gateway;
          port = config.infra.clients.container-komga.ports.default;
          # JVM: cap reclaim pressure rather than let the heap grow into the host.
          memoryHigh = "2G";

          # Single sign-on via Authelia. Spring Security drives the whole flow,
          # so this is plain Spring OAuth2 client config rather than anything
          # Komga-specific; oauth2-account-creation lets a first-time Authelia
          # login create the local account instead of 403ing. The client secret
          # arrives as an env var (envFile below) — Spring resolves ${…}
          # placeholders from the environment, so it stays out of the nix store.
          extraSettings = {
            komga.oauth2-account-creation = true;
            spring.security.oauth2.client = {
              registration.authelia = {
                client-id = "komga";
                client-secret = "\${KOMGA_OIDC_CLIENT_SECRET}";
                client-name = "Authelia";
                scope = "openid,profile,email";
                authorization-grant-type = "authorization_code";
                # Spring's fixed callback shape; must match the redirect_uri
                # registered in authelia.nix.
                redirect-uri = "{baseUrl}/login/oauth2/code/{registrationId}";
              };
              provider.authelia = {
                issuer-uri = "https://authelia.caddy.${config.infra.vlans.apps.domain}";
                user-name-attribute = "preferred_username";
              };
            };
          };

          configuratarr = {
            enable = true;
            waitForHealthy = true;
            settings.komga = {
              type = "komga-v1";
              url = "http://localhost:${toString config.infra.clients.container-komga.ports.default}";
              api_key = "\${env.KOMGA_API_KEY}";
              libraries = [
                {
                  name = "Comics";
                  root = "/mnt/media/library/comics";
                  # ⚠ SEEDING SAFETY — both stay false. The library holds
                  # hardlinks to torrent data qBittorrent is still seeding in
                  # /mnt/media/complete. Conversion emits a NEW file and drops
                  # the link (silently doubling disk), and MAM comics are
                  # predominantly .cbr, so it would fire on nearly every import.
                  convert_to_cbz = false;
                  repair_extensions = false;
                  # Read-only metadata sources: ComicInfo.xml inside the archive
                  # and artwork alongside it. Komga keeps what it derives in its
                  # own database and never writes back into the files.
                  scan_cbx = true;
                  import_comic_info_book = true;
                  import_comic_info_series = true;
                  import_local_artwork = true;
                  hash_files = true;
                }
              ];
            };
          };

          bindMounts = {
            "/var/lib/komga" = {
              hostPath = "/var/lib/appdata/komga";
              isReadOnly = false;
            };
            # Komga only ever reads the comic files; its own database and
            # thumbnails live in /var/lib/komga above. The library path itself is
            # added in the UI — point it at /mnt/media/library/comics.
            "/mnt/media" = {
              hostPath = "/mnt/media";
              isReadOnly = false;
            };
          };
          envFile = config.sops.templates."komga-env".path;
          nameservers = [
            config.infra.clients.container-dns.ip
            "1.1.1.1"
          ];
          logging = {
            enable = true;
            lokiAddress = "http://${config.infra.clients.container-loki.ip}:${toString config.infra.clients.container-loki.ports.default}";
            prometheusAddress = "http://${config.infra.clients.container-prometheus.ip}:${toString config.infra.clients.container-prometheus.ports.default}/api/v1/write";
            tempoAddress = "http://${config.infra.clients.container-tempo.ip}:${toString config.infra.clients.container-tempo.ports.otlp-http}/v1/traces";
            # Komga ships spring-boot-starter-actuator but NOT
            # micrometer-registry-prometheus — its OpenAPI exposes only
            # /actuator/info, so there is no /actuator/prometheus to scrape.
            # Everything below is derived from the journal instead. Counters reset
            # when Vector restarts, so query them with increase().
            prometheusInputs = [
              "komga_scan_metrics"
              "komga_warn_metrics"
            ];
            extraSettings.transforms = {
              # Spring Boot logs one line per completed library scan.
              komga_scan_filter = {
                type = "filter";
                inputs = [ "journal" ];
                condition = ''match(to_string(.message) ?? "", r'(?i)scan.*(complete|finished)')'';
              };
              komga_scan_metrics = {
                type = "log_to_metric";
                inputs = [ "komga_scan_filter" ];
                metrics = [
                  {
                    type = "counter";
                    field = "message";
                    name = "komga_scans_total";
                  }
                ];
              };
              komga_warn_filter = {
                type = "filter";
                inputs = [ "journal" ];
                condition = ''match(to_string(.message) ?? "", r'\bWARN\b|\bERROR\b')'';
              };
              komga_warn_metrics = {
                type = "log_to_metric";
                inputs = [ "komga_warn_filter" ];
                metrics = [
                  {
                    type = "counter";
                    field = "message";
                    name = "komga_warnings_total";
                  }
                ];
              };
            };
          };
        };

        systemd.tmpfiles.rules = [
          "d /var/lib/appdata/komga 0770 ${toString cfg.uid} ${toString cfg.gid} - -"
          "Z /var/lib/appdata/komga 0770 ${toString cfg.uid} ${toString cfg.gid} - -"
        ];

        # Spring resolves the OIDC issuer by fetching its discovery document at
        # start-up and refuses to build the security context if that fails, so
        # Komga crash-loops until Authelia answers. Caddy and Authelia both gate
        # container-wave-core, so ordering after it avoids the race.
        systemd.services."container@komga" = {
          after = [ "container-wave-core.target" ];
          wants = [ "container@authelia.service" ];
        };
      };
  };
}
