/*
  seasonpackarr — host configuration override for seasonpackarr container

  Pinned to upstream v0.16.0, whose config shape is `preImportPath` +
  `parseTorrentFile`. The per-client `import` policy documented on upstream's
  develop branch is unreleased — when it ships, preImportPath below becomes
  clients.default.import.savePath (or .category) and the qBittorrent action in
  autobrr/filters/_seasonpackarr.nix goes away, since the new version adds the
  torrent itself.

  Why only the token comes from the environment: v0.16.0's env override parser
  splits CLIENTS_<NAME>_<SETTING> on every underscore and only accepts a
  two-part result, so CLIENTS_DEFAULT_PRE_IMPORT_PATH is silently dropped.
  Everything except flat top-level keys has to come from the config file.

  No credentials for qBittorrent: this container's media-VLAN IP is in
  qBittorrent's authSubnetWhitelist, and go-qbittorrent's Login() returns early
  when username and password are both empty — same arrangement cross-seed uses.
*/
{ den, ... }:
{
  den.aspects.hosts._.server._.containers._.seasonpackarr = {
    aspect-name = "hosts.server.containers.seasonpackarr";
    includes = [
      den.aspects.containers._.seasonpackarr
      den.aspects.infra._.clients._.container-seasonpackarr
      den.aspects.infra._.clients._.container-qbittorrent
      den.aspects.infra._.clients._.container-dns
      den.aspects.infra._.clients._.container-mosquitto
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
        cl = config.infra.clients;
        qbit = cl.container-qbittorrent;
        # Reads the infra client rather than cfg.port: cfg is the option set
        # this module is defining, so going through it here would be a cycle.
        exporter = import ./_exporter.nix {
          port = cl.container-seasonpackarr.ports.default;
        };
        # Same path as the `seasonpacks` qBittorrent category (see
        # qbittorrent/qbittorrent.nix): seasonpackarr hardlinks the matched
        # episodes in here, and autobrr's qBittorrent action adds the pack with
        # that category so the client looks in the same place.
        preImportPath = "/mnt/media/complete/seasonpacks";
      in
      {
        containerConfigs.seasonpackarr = {
          lldpd.enable = true;
          hostBridge = "br-media";
          macAddress = cl.container-seasonpackarr.mac;
          localAddress = "${cl.container-seasonpackarr.ip}/${toString config.infra.vlans.media.subnetPrefix}";
          gateway = config.infra.vlans.media.staticAddresses.gateway;
          port = cl.container-seasonpackarr.ports.default;
          envFile = config.sops.templates."seasonpackarr-env".path;
          inherit preImportPath;
          qbittorrentHost = qbit.ip;
          qbittorrentPort = qbit.ports.webui;
          smartMode = true;
          smartModeThreshold = 0.51;
          # A REPACK episode is still byte-identical to its counterpart in a
          # non-repacked pack; without this the whole pack is rejected.
          fuzzyMatching.skipRepackCompare = true;
          # seasonpackarr POSTs its notifications here (loopback, the Vector
          # sidecar) instead of to Discord; see notificationWebhook.
          notificationWebhook = "http://127.0.0.1:9001/";
          # INFO adds the per-announce rejection reasons, which is how the match
          # rate gets judged before deciding whether TVDB is worth adding:
          # metadata lookups feed smartMode, and a failure returns a non-250 from
          # /api/pack, so the filter simply does not match and the announce falls
          # through to the normal TV filter for Sonarr to grab as usual.
          notificationLevel = [
            "MATCH"
            "INFO"
            "ERROR"
          ];
          bindMounts = {
            # Both sides of every hardlink live here: qBittorrent's download
            # data and the season pack folder seasonpackarr assembles.
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
            inherit (exporter) prometheusInputs;
            # seasonpackarr POSTs its notifications to the local Vector sidecar
            # (loopback); republished verbatim to MQTT so HA consumes via an
            # mqtt trigger, and read by _exporter.nix for pack-outcome metrics.
            # Same arrangement cross-seed uses.
            extraSettings = lib.recursiveUpdate exporter.extraSettings {
              sources.webhook_in = {
                type = "http_server";
                address = "127.0.0.1:9001";
                method = "POST";
                path = "/";
                decoding.codec = "bytes"; # keep the embed JSON verbatim → .message
                framing.method = "bytes"; # one event per request body, not per line
              };
              sinks.webhook_mqtt = {
                type = "mqtt";
                inputs = [ "webhook_in" ];
                host = cl.container-mosquitto.ip;
                port = cl.container-mosquitto.ports.default;
                client_id = "vector-seasonpackarr"; # unique per container → no broker client-id collision
                topic = "homelab/notify/seasonpackarr";
                quality_of_service = "atleastonce";
                retain = false;
                encoding.codec = "text"; # emit .message raw (the embed JSON)
              };
            };
          };
        };
      };
  };
}
