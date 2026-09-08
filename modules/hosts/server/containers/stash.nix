# stash — host configuration override for stash container
{ den, ... }:
{
  den.aspects.hosts._.server._.containers._.stash = {
    aspect-name = "hosts.server.containers.stash";
    includes = [
      den.aspects.containers._.stash
      den.aspects.infra._.clients._.container-stash
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
        cfg = config.containerConfigs.stash;
      in
      {
        containerConfigs.stash = {

          lldpd.enable = true;
          hostBridge = "br-media";
          macAddress = config.infra.clients.container-stash.mac;
          localAddress = "${config.infra.clients.container-stash.ip}/${toString config.infra.vlans.media.subnetPrefix}";
          gateway = config.infra.vlans.media.staticAddresses.gateway;
          port = config.infra.clients.container-stash.ports.default;
          bindMounts = {
            # SQLite db plus generated/, cache/ and blobs/. Previews and sprites
            # are generated per scene, so this grows well past typical appdata —
            # point it at /mnt/media if the local disk becomes the constraint.
            "/var/lib/stash" = {
              hostPath = "/var/lib/appdata/stash";
              isReadOnly = false;
            };
            # Read-only on purpose: Whisparr owns this tree and hardlink-imports
            # into it. A writable mount would let Stash's organize/rename move
            # files out from under Whisparr's file records.
            "/mnt/media/library/xxx" = {
              hostPath = "/mnt/media/library/xxx";
              isReadOnly = true;
            };
            # The download tree, also read-only (qBittorrent is still seeding
            # these). Indexing it decouples identification from Whisparr: files
            # it never matches would otherwise sit here unseen forever. Once
            # Whisparr does hardlink one into library/, it is the same inode and
            # therefore the same oshash, so Stash attaches it to the existing
            # scene rather than duplicating it.
            "/mnt/media/complete/xxx" = {
              hostPath = "/mnt/media/complete/xxx";
              isReadOnly = true;
            };
          };
          libraryPaths = [
            "/mnt/media/library/xxx"
            "/mnt/media/complete/xxx"
          ];
          # Each worker runs its own ffmpeg; 4 keeps generate off a single core
          # without swamping the box during a big scan.
          parallelTasks = 4;
          taskInterval = "hourly";
          jwtSecretKeyFile = config.sops.secrets.stash-jwt-secret.path;
          sessionStoreKeyFile = config.sops.secrets.stash-session-key.path;
          stashdbApiKeyFile = config.sops.secrets.stashdb-apikey.path;
          nameservers = [
            config.infra.clients.container-dns.ip
            "1.1.1.1"
          ];
          logging = {
            enable = true;
            lokiAddress = "http://${config.infra.clients.container-loki.ip}:${toString config.infra.clients.container-loki.ports.default}";
            prometheusAddress = "http://${config.infra.clients.container-prometheus.ip}:${toString config.infra.clients.container-prometheus.ports.default}/api/v1/write";
            tempoAddress = "http://${config.infra.clients.container-tempo.ip}:${toString config.infra.clients.container-tempo.ports.otlp-http}/v1/traces";
          };
        };

        systemd.tmpfiles.rules = [
          "d /var/lib/appdata/stash 0770 ${toString cfg.uid} ${toString cfg.gid} - -"
          "Z /var/lib/appdata/stash 0770 ${toString cfg.uid} ${toString cfg.gid} - -"
        ];

        systemd.services."container@stash" = {
          before = [ "container-wave-downstream.target" ];
          after = [
            "remote-fs.target"
            "container-wave-arr.target"
          ];
        };
      };
  };
}
