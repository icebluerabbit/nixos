# jellyfin — host configuration override for jellyfin container
{ den, ... }:
{
  den.aspects.hosts._.server._.containers._.jellyfin = {
    aspect-name = "hosts.server.containers.jellyfin";
    includes = [
      den.aspects.containers._.jellyfin
      den.aspects.infra._.clients._.container-jellyfin
      den.aspects.infra._.clients._.container-mosquitto
      den.aspects.infra._.clients._.container-dns
      den.aspects.infra._.networks._.media
      den.aspects.hosts._.server._.security
      den.aspects.hosts._.server._.networking._.vlans._.media
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
        cfg = config.containerConfigs.jellyfin;
      in
      {
        containerConfigs.jellyfin = {

          lldpd.enable = true;
          port = config.infra.clients.container-jellyfin.ports.http;
          hostBridge = "br-media";
          macAddress = config.infra.clients.container-jellyfin.mac;
          localAddress = "${config.infra.clients.container-jellyfin.ip}/${toString config.infra.vlans.media.subnetPrefix}";
          gateway = config.infra.vlans.media.staticAddresses.gateway;
          bindMounts = {
            "/var/lib/jellyfin" = {
              hostPath = "/var/lib/appdata/jellyfin";
              isReadOnly = false;
            };
            "/mnt/media" = {
              hostPath = "/mnt/media";
              isReadOnly = false;
            };
            "/run/secrets/jellyfin-env" = {
              hostPath = config.sops.templates."jellyfin-env".path;
              isReadOnly = true;
            };
          };
          envFile = config.sops.templates."jellyfin-env".path;
          configuratarr = {
            enable = true;
            waitForHealthy = true;
            settings.jellyfin = {
              type = "jellyfin-v11";
              url = "http://localhost:${toString config.infra.clients.container-jellyfin.ports.http}";
              api_key = "\${env.JELLYFIN_API_KEY}";
              encoding_options = {
                hardware_acceleration_type = "qsv";
                vaapi_device = "/dev/dri/renderD128";
                enable_hardware_encoding = true;
                allow_hevc_encoding = true;
                allow_av1_encoding = true;
                # VPP tonemapping is HDR10-only and cannot reshape Dolby Vision.
                # Without OpenCL tonemapping Jellyfin falls back to tagging DV
                # Profile 5 frames as bt709 via setparams and encoding the raw
                # IPT-PQ-c2 data unconverted — the browser reads IPT as YCbCr
                # and renders a purple/green tint. OpenCL tonemapping handles
                # DV; VA-API<->OpenCL sharing is zero-copy on this Arc GPU.
                enable_tonemapping = true;
                enable_vpp_tonemapping = true;
                # Browsers cannot direct-play mkv, so every play spawns ffmpeg
                # writing HLS segments. The default cache lives on the container's
                # ephemeral tmpfs root (23G of RAM); a 4K remux direct-streams at
                # ~46x realtime and fills it in minutes, after which every segment
                # is written 0-byte and the client reports a fatal HLS error.
                # Park the segments on the appdata nvme instead (excluded from the
                # restic job in hosts/server/backups.nix), and bound how far ffmpeg
                # may run ahead of playback so no single stream can run away again.
                transcoding_temp_path = "/var/lib/jellyfin/transcodes";
                enable_throttling = true;
                throttle_delay_seconds = 180;
                enable_segment_deletion = true;
                segment_keep_seconds = 720;
                hardware_decoding_codecs = [
                  "h264"
                  "hevc"
                  "vc1"
                  "av1"
                  "vp8"
                  "vp9"
                  "mpeg2video"
                ];
              };
              libraries = [
                {
                  name = "Movies";
                  collection_type = "movies";
                  paths = [ "/mnt/media/library/movies" ];
                }
                {
                  name = "Shows";
                  collection_type = "tvshows";
                  paths = [ "/mnt/media/library/tv" ];
                }
                {
                  name = "Music";
                  collection_type = "music";
                  paths = [ "/mnt/media/library/music" ];
                }
                {
                  name = "YouTube";
                  collection_type = "homevideos";
                  paths = [ "/mnt/media/complete/youtube" ];
                }
              ];
            };
          };
          nameservers = [
            config.infra.clients.container-dns.ip
            "1.1.1.1"
          ];
          acceleration = {
            enable = true;
            devices = [
              "/dev/dri/card0"
              "/dev/dri/renderD128"
            ];
            extraPackages = with pkgs; [
              intel-media-driver
              vpl-gpu-rt
              intel-compute-runtime
            ];
            libvaDriverName = "iHD";
          };
          logging = {
            enable = true;
            lokiAddress = "http://${config.infra.clients.container-loki.ip}:${toString config.infra.clients.container-loki.ports.default}";
            prometheusAddress = "http://${config.infra.clients.container-prometheus.ip}:${toString config.infra.clients.container-prometheus.ports.default}/api/v1/write";
            tempoAddress = "http://${config.infra.clients.container-tempo.ip}:${toString config.infra.clients.container-tempo.ports.otlp-http}/v1/traces";
            # Vector polls the Jellyfin API for metrics (no packaged exporter);
            # X-Emby-Token comes from JELLYFIN_API_KEY in the mounted jellyfin-env.
            environmentFile = "/run/secrets/jellyfin-env";
            prometheusInputs = [
              "jellyfin_counts_metrics"
              "jellyfin_sessions_metrics"
              "jellyfin_up_metrics"
            ];
            extraSettings =
              lib.recursiveUpdate
                (import ./_exporter.nix {
                  port = config.infra.clients.container-jellyfin.ports.http;
                })
                {
                  # Jellyfin's Webhook plugin POSTs here on loopback (set the plugin's
                  # Generic Destination URL to http://127.0.0.1:9001/); the sidecar
                  # republishes verbatim to MQTT so HA consumes via an mqtt trigger.
                  sources.webhook_in = {
                    type = "http_server";
                    address = "127.0.0.1:9001";
                    method = "POST";
                    path = "/";
                    decoding.codec = "bytes"; # keep Jellyfin's JSON body verbatim → .message
                    framing.method = "bytes"; # one event per request body, not per line
                  };
                  sinks.webhook_mqtt = {
                    type = "mqtt";
                    inputs = [ "webhook_in" ];
                    host = config.infra.clients.container-mosquitto.ip;
                    port = config.infra.clients.container-mosquitto.ports.default;
                    client_id = "vector-jellyfin"; # unique per container → no broker client-id collision
                    topic = "homelab/notify/jellyfin";
                    quality_of_service = "atleastonce";
                    retain = false;
                    encoding.codec = "text"; # emit .message raw (the Jellyfin JSON)
                  };
                };
          };
        };

        systemd.tmpfiles.rules = [
          "d /var/lib/appdata/jellyfin 0770 ${toString cfg.uid} ${toString cfg.gid} - -"
          "Z /var/lib/appdata/jellyfin 0770 ${toString cfg.uid} ${toString cfg.gid} - -"
        ];

        systemd.services."container@jellyfin" = {
          before = [ "container-wave-apps.target" ];
          after = [
            "remote-fs.target"
            "container-wave-core.target"
          ];
        };
      };
  };
}
