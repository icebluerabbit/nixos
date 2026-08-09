# cleanuparr — host configuration override for cleanuparr container
{ den, ... }:
{
  den.aspects.hosts._.server._.containers._.cleanuparr = {
    aspect-name = "hosts.server.containers.cleanuparr";
    includes = [
      den.aspects.containers._.cleanuparr
      den.aspects.infra._.clients._.container-cleanuparr
      den.aspects.infra._.clients._.container-dns
      den.aspects.infra._.clients._.container-postgres
      den.aspects.infra._.clients._.container-qbittorrent
      den.aspects.infra._.clients._.container-sonarr
      den.aspects.infra._.clients._.container-radarr
      den.aspects.infra._.clients._.container-lidarr
      den.aspects.infra._.networks._.apps
      den.aspects.infra._.networks._.media
      den.aspects.hosts._.server._.security
      den.aspects.hosts._.server._.networking._.vlans._.media
      den.aspects.infra._.clients._.container-loki
      den.aspects.infra._.clients._.container-prometheus
      den.aspects.infra._.clients._.container-tempo
    ];

    nixos =
      { config, lib, ... }:
      let
        cfg = config.containerConfigs.cleanuparr;
        cl = config.infra.clients;
        port = cl.container-cleanuparr.ports.default;
        apps = config.infra.vlans.apps;
        # Every category qBittorrent defines (see qbittorrent/_bittorrent.nix) —
        # the unlinked/dead scans are scoped to downloads the *arr apps own.
        categories = [
          "tv"
          "movies"
          "books"
          "audiobooks"
          "music"
          "misc"
        ];
        arr = name: version: {
          inherit name;
          enabled = true;
          url = "http://${cl."container-${name}".ip}:${toString cl."container-${name}".ports.default}";
          api_key = "\${env.${lib.toUpper name}_APIKEY}";
          inherit version;
        };
      in
      {
        systemd.services."container@cleanuparr" = {
          wants = [ "container@postgresql.service" ];
          after = [ "container@postgresql.service" ];
        };

        containerConfigs.cleanuparr = {
          lldpd.enable = true;
          hostBridge = "br-media";
          macAddress = cl.container-cleanuparr.mac;
          localAddress = "${cl.container-cleanuparr.ip}/${toString config.infra.vlans.media.subnetPrefix}";
          gateway = config.infra.vlans.media.staticAddresses.gateway;
          inherit port;
          dbHost = cl.container-postgres.ip;
          dbPort = cl.container-postgres.ports.default;
          mediaDir = "/mnt/media";
          envFile = config.sops.templates."cleanuparr-env".path;
          configuratarr = {
            enable = true;
            waitForHealthy = true;
            settings.cleanuparr = {
              type = "cleanuparr-v1";
              url = "http://localhost:${toString port}";
              api_key = "\${env.CLEANUPARR_API_KEY}";

              general = {
                display_support_banner = false;
                dry_run = false;
                status_check_enabled = true;
                # Caddy fronts the UI: trust its forwarded client address and treat
                # the apps VLAN as local, so the dashboard needs no second login.
                auth = {
                  disable_auth_for_local_addresses = true;
                  trust_forwarded_headers = true;
                  trusted_networks = [ "${apps.subnet}/${toString apps.subnetPrefix}" ];
                };
              };

              # Strike-and-report only. Nothing here deletes a torrent or a file —
              # the destructive resources (orphaned_files_config, seeding_rules,
              # slow_rules) are deliberately left undeclared.
              queue_cleaner = {
                enabled = true;
                failed_import = {
                  max_strikes = 3;
                  # Exclude with an empty pattern list means "strike every failed
                  # import". Include is the stored default and Cleanuparr rejects it
                  # outright with no patterns ("At least one pattern must be
                  # specified when using the Include pattern mode").
                  pattern_mode = "Exclude";
                  patterns = [ ];
                  delete_private = false;
                };
                downloading_metadata_max_strikes = 5;
              };
              download_cleaner.enabled = true;

              # `version` only picks a client shape for Whisparr (see Cleanuparr's
              # ArrClientFactory); for these four it is the *arr API version.
              # readarr is deliberately absent: it is the one *arr with no
              # configuratarr config of its own, and the packaged 0.4.18 is from
              # the archived upstream. Books go through lazylibrarian.
              sonarr_instances = [ (arr "sonarr" 3) ];
              radarr_instances = [ (arr "radarr" 3) ];
              lidarr_instances = [ (arr "lidarr" 1) ];

              stall_rules = [
                {
                  name = "stalled";
                  enabled = true;
                  max_strikes = 5;
                  privacy_type = "Both";
                  # Private-tracker torrents are struck and re-grabbed, never pulled
                  # out from under the client — H&R and seed-forever depend on them
                  # staying put.
                  delete_private_torrents_from_client = false;
                  change_category = false;
                  reset_strikes_on_progress = true;
                }
              ];

              download_clients = [
                {
                  name = "qbittorrent";
                  enabled = true;
                  type_name = "qBittorrent";
                  protocol = "Torrent";
                  # No credentials: the media VLAN is in qBittorrent's
                  # authSubnetWhitelist. Paths need no translation either — /mnt/media
                  # is bind-mounted here at the same path qBittorrent reports.
                  host = "http://${cl.container-qbittorrent.ip}:${toString cl.container-qbittorrent.ports.webui}";
                  # Tag, never recategorise: a new category has no save path, so
                  # qBittorrent would relocate the data under autoTMM. A tag moves
                  # nothing — these two only mark downloads for a human to review.
                  unlinked_config = {
                    enabled = true;
                    use_tag = true;
                    target_category = "cleanuparr-unlinked";
                    inherit categories;
                  };
                  dead_torrent_config = {
                    enabled = true;
                    use_tag = true;
                    target_category = "cleanuparr-dead";
                    max_strikes = 5;
                    inherit categories;
                  };
                }
              ];
            };
          };
          bindMounts = {
            "/var/lib/cleanuparr" = {
              hostPath = "/var/lib/appdata/cleanuparr";
              isReadOnly = false;
            };
            # Unlinked/orphaned download handling stats the real tree qBittorrent
            # and the *arr apps write to.
            "/mnt/media" = {
              hostPath = "/mnt/media";
              isReadOnly = false;
            };
          };
          nameservers = [
            cl.container-dns.ip
            "1.1.1.1"
          ];
          logging = {
            enable = true;
            lokiAddress = "http://${cl.container-loki.ip}:${toString cl.container-loki.ports.default}";
            prometheusAddress = "http://${cl.container-prometheus.ip}:${toString cl.container-prometheus.ports.default}/api/v1/write";
            tempoAddress = "http://${cl.container-tempo.ip}:${toString cl.container-tempo.ports.otlp-http}/v1/traces";
          };
        };

        systemd.tmpfiles.rules = [
          "d /var/lib/appdata/cleanuparr 0700 ${toString cfg.uid} ${toString cfg.gid} - -"
          "Z /var/lib/appdata/cleanuparr 0700 ${toString cfg.uid} ${toString cfg.gid} - -"
        ];
      };
  };
}
