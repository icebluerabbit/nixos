# cross-seed — host configuration override for cross-seed container
{ den, ... }:
{
  den.aspects.hosts._.server._.containers._.cross-seed = {
    aspect-name = "hosts.server.containers.cross-seed";
    includes = [
      den.aspects.containers._.cross-seed
      den.aspects.infra._.clients._.container-cross-seed
      den.aspects.infra._.clients._.container-prowlarr
      den.aspects.infra._.clients._.container-sonarr
      den.aspects.infra._.clients._.container-radarr
      den.aspects.infra._.clients._.container-qbittorrent
      den.aspects.infra._.clients._.container-mosquitto
      den.aspects.infra._.clients._.container-dns
      den.aspects.infra._.networks._.media
      den.aspects.infra._.networks._.download
      den.aspects.hosts._.server._.security
      den.aspects.hosts._.server._.networking._.vlans._.media
      den.aspects.infra._.clients._.container-loki
      den.aspects.infra._.clients._.container-prometheus
      den.aspects.infra._.clients._.container-tempo
    ];

    nixos =
      { config, lib, ... }:
      let
        cfg = config.containerConfigs.cross-seed;
        prowlarr = config.infra.clients.container-prowlarr;
        sonarr = config.infra.clients.container-sonarr;
        radarr = config.infra.clients.container-radarr;
        qbit = config.infra.clients.container-qbittorrent;
        # Reads the infra client rather than cfg.port: cfg is the option set
        # this module is defining, so going through it here would be a cycle.
        exporter = import ./_exporter.nix {
          port = config.infra.clients.container-cross-seed.ports.default;
        };
      in
      {
        # cross-seed's own daemon API key + the secret indexer/client URLs
        # (Torznab + *arr keys). Kept out of the Nix store: rendered by sops into
        # settingsFile and merged over the non-secret `settings` at runtime.
        sops = {
          secrets.cross-seed-apikey = { };
          templates."cross-seed-settings" = {
            restartUnits = [ "container@cross-seed.service" ];
            content = builtins.toJSON {
              apiKey = config.sops.placeholder.cross-seed-apikey;
              # Prowlarr per-indexer Torznab feeds (id 1 = PreToMe, 2 = TorrentLeech,
              # 3 = DigitalCore, 4 = RetroToon, 5 = DarkPeers, 6 = YU-Scene). Only *enabled* indexers belong here:
              # Prowlarr answers a disabled indexer's feed with 410 Gone, which fails
              # every updateIndexerCaps run.
              torznab = [
                "http://${prowlarr.ip}:${toString prowlarr.ports.default}/1/api?apikey=${config.sops.placeholder.prowlarr-apikey}"
                "http://${prowlarr.ip}:${toString prowlarr.ports.default}/2/api?apikey=${config.sops.placeholder.prowlarr-apikey}"
                "http://${prowlarr.ip}:${toString prowlarr.ports.default}/3/api?apikey=${config.sops.placeholder.prowlarr-apikey}"
                "http://${prowlarr.ip}:${toString prowlarr.ports.default}/4/api?apikey=${config.sops.placeholder.prowlarr-apikey}"
                "http://${prowlarr.ip}:${toString prowlarr.ports.default}/5/api?apikey=${config.sops.placeholder.prowlarr-apikey}"
                "http://${prowlarr.ip}:${toString prowlarr.ports.default}/6/api?apikey=${config.sops.placeholder.prowlarr-apikey}"
              ];
              # No creds: cross-seed's media-VLAN IP is in qBittorrent's
              # authSubnetWhitelist, so its WebUI bypasses auth for us.
              torrentClients = [
                "qbittorrent:http://${qbit.ip}:${toString qbit.ports.webui}"
              ];
              sonarr = [
                "http://${sonarr.ip}:${toString sonarr.ports.default}?apikey=${config.sops.placeholder.sonarr-apikey}"
              ];
              radarr = [
                "http://${radarr.ip}:${toString radarr.ports.default}?apikey=${config.sops.placeholder.radarr-apikey}"
              ];
            };
          };
        };

        containerConfigs.cross-seed = {
          lldpd.enable = true;
          hostBridge = "br-media";
          macAddress = config.infra.clients.container-cross-seed.mac;
          localAddress = "${config.infra.clients.container-cross-seed.ip}/${toString config.infra.vlans.media.subnetPrefix}";
          gateway = config.infra.vlans.media.staticAddresses.gateway;
          port = config.infra.clients.container-cross-seed.ports.default;
          configDir = "/var/lib/cross-seed";
          settingsFile = "/run/secrets/cross-seed-settings";
          settings = {
            port = config.infra.clients.container-cross-seed.ports.default;
            # Inject cross-seeds straight into qBittorrent, hardlinking existing
            # data (all on the one /mnt/media volume) so no re-download.
            action = "inject";
            linkType = "hardlink";
            linkDirs = [ "/mnt/media/cross-seed/links" ];
            dataDirs = [
              "/mnt/media/complete/movies"
              "/mnt/media/complete/tv"
              "/mnt/media/complete/music"
              "/mnt/media/complete/books"
              "/mnt/media/complete/misc"
              "/mnt/media/download"
            ];
            # null: torrents are enumerated via the qBittorrent client integration
            # (torrentClients). qbt's real .torrent store lives in another container,
            # and pointing torrentDir at the flat export dir fails cross-seed's
            # client-vs-dir validation ("Invalid torrentDir").
            torrentDir = null;
            useClientTorrents = true;
            matchMode = "safe";
            includeSingleEpisodes = true;
            # Both webhook triggers are edge-triggered on *new* things: autobrr's
            # /api/announce fires only on a grab, qbt's /api/webhook only on a
            # completion. Nothing re-examines the back catalogue, so anything
            # already seeding when a tracker gained the content stays unmatched
            # forever. searchCadence is what closes that.
            #
            # excludeOlder can't be null while searchCadence is set — upstream
            # requires it be 2-5x excludeRecentSearch (configSchema.js) and dies
            # at startup otherwise. It measures first-seen-by-cross-seed, not
            # data age, and both bounds are tracked per (searchee, indexer)
            # pair: an indexer with no timestamp row for a searchee is always
            # eligible ("Don't exclude if new indexer was added", preFilter.js),
            # so adding a tracker still sweeps the whole back catalogue against
            # it. The bounds only cap re-searching pairs already covered.
            #
            # The ratio cap fixes the budget at 5 searches per pair either way,
            # so the only real choice is how far to spread them. Arrivals are
            # already covered in real time by the two webhooks and rssCadence —
            # this sweep exists for the long tail, i.e. someone uploading a
            # years-old rip to a tracker we already searched. Hence the wide
            # window: upstream's own "spread out over a long period" pairing.
            searchCadence = "1 day";
            excludeRecentSearch = "90 days";
            excludeOlder = "450 days";
            # Poll the four Torznab feeds for new uploads and match them against
            # what we already hold: catches a release landing on DigitalCore days
            # after we took it from TorrentLeech, without waiting for that
            # torrent's next sweep slot. Minimum accepted cadence is 10 minutes.
            rssCadence = "30 minutes";
            # Seconds between searches (upstream default 10). The daily sweep is
            # the first thing here to generate sustained query volume, and it
            # fans out across four private indexers behind one Prowlarr — 30s
            # keeps it under their limits. Raise excludeRecentSearch rather than
            # lowering this if the sweep stops finishing within a day.
            delay = 30;
            # null recommended when action=inject (no .torrent files emitted to disk).
            outputDir = null;
            # cross-seed POSTs its own result/inject notifications ({title,body,extra})
            # here → HA webhook → #cross-seed. Distinct from autobrr's announce POST-ack.
            notificationWebhookUrls = [
              "http://127.0.0.1:9001/"
            ];
          };
          bindMounts = {
            "/var/lib/cross-seed" = {
              hostPath = "/var/lib/appdata/cross-seed";
              isReadOnly = false;
            };
            "/mnt/media" = {
              hostPath = "/mnt/media";
              isReadOnly = false;
            };
            "/run/secrets/cross-seed-settings" = {
              hostPath = config.sops.templates."cross-seed-settings".path;
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
            inherit (exporter) prometheusInputs;
            # cross-seed POSTs its result/inject notifications to the local Vector
            # sidecar (loopback); republished verbatim to MQTT so HA consumes via an
            # mqtt trigger instead of a direct webhook.
            extraSettings = lib.recursiveUpdate exporter.extraSettings {
              sources.webhook_in = {
                type = "http_server";
                address = "127.0.0.1:9001";
                method = "POST";
                path = "/";
                decoding.codec = "bytes"; # keep cross-seed's JSON body verbatim → .message
                framing.method = "bytes"; # one event per request body, not per line
              };
              sinks.webhook_mqtt = {
                type = "mqtt";
                inputs = [ "webhook_in" ];
                host = config.infra.clients.container-mosquitto.ip;
                port = config.infra.clients.container-mosquitto.ports.default;
                client_id = "vector-cross-seed"; # unique per container → no broker client-id collision
                topic = "homelab/notify/cross-seed";
                quality_of_service = "atleastonce";
                retain = false;
                encoding.codec = "text"; # emit .message raw (the cross-seed JSON)
              };
            };
          };
        };

        systemd.tmpfiles.rules = [
          "d /var/lib/appdata/cross-seed 0770 ${toString cfg.uid} ${toString cfg.gid} - -"
          "Z /var/lib/appdata/cross-seed 0770 ${toString cfg.uid} ${toString cfg.gid} - -"
          "d /mnt/media/cross-seed 0770 ${toString cfg.uid} ${toString cfg.gid} - -"
          "d /mnt/media/cross-seed/links 0770 ${toString cfg.uid} ${toString cfg.gid} - -"
        ];

        systemd.services."container@cross-seed" = {
          before = [ "container-wave-downstream.target" ];
          after = [
            "remote-fs.target"
            "container-wave-arr.target"
          ];
        };
      };
  };
}
