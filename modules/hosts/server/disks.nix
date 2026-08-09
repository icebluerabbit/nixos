# disks — basic disk partitioning and filesystem layout configuration for server host
{ den, ... }:
{
  den.aspects.hosts._.server._.disks = {
    aspect-name = "hosts.server.disks";
    includes = [
      den.aspects.boot._.disko
      den.aspects.infra._.clients._.unas-pro
    ];

    nixos =
      { config, pkgs, ... }:
      {
        fileSystems = {
          "/mnt/media" = {
            device = "${config.infra.clients.unas-pro.ip}:/var/nfs/shared/Media";
            fsType = "nfs";
            options = [
              # The UNAS is on the storage VLAN, so these mounts ride br-storage. Without
              # this, the mount unit only orders on network-online.target, and a mount that
              # (re)connects while br-storage has no address falls through to the default
              # route on enp9s0 and binds 10.10.10.2 as its source for the life of the
              # transport — asymmetric, and the mgmt NIC's MAC never gets learned.
              "x-systemd.requires=network-addresses-br-storage.service"
              "x-systemd.after=network-addresses-br-storage.service"
              "x-systemd.automount"
              "noauto"
              "nfsvers=3"
              "rsize=1048576"
              "wsize=1048576"
              # nconnect is PER client↔server TRANSPORT, fixed by the first mount to the
              # server. All three shares below hit 10.10.50.222, so they share ONE
              # transport — every mount must agree on nconnect or the first-mounted wins
              # (kernel silently ignores later differing values). Keep all at 16.
              "nconnect=16"
              "noatime"
              "nodiratime"
              "actimeo=3600"
              "hard"
              "timeo=600"
              "retrans=2"
              # FS-Cache: cache read pages on the local nvme (services.cachefilesd →
              # /var/cache/fscache subvol) so repeat seeding reads of popular pieces hit
              # SSD instead of seeking the HDD-backed UNAS. Completed media is read-only
              # and immutable, so cache coherence is a non-issue. ONLY /mnt/media (the
              # seed set) is cached — data/projects change too often to cache safely.
              "fsc"
              # Order after cachefilesd so caching is active at mount time. `wants` not
              # `requires`: a dead cache drive must never block the media mount.
              "x-systemd.wants=cachefilesd.service"
              "x-systemd.after=cachefilesd.service"
            ];
          };
          "/mnt/data" = {
            device = "${config.infra.clients.unas-pro.ip}:/var/nfs/shared/Data";
            fsType = "nfs";
            options = [
              "x-systemd.requires=network-addresses-br-storage.service" # see /mnt/media
              "x-systemd.after=network-addresses-br-storage.service"
              "x-systemd.automount"
              "noauto"
              "nfsvers=3"
              "rsize=65536"
              "wsize=65536"
              "nconnect=16" # shared transport w/ /mnt/media — must match (see above)
              "noatime"
              "nodiratime"
              "actimeo=30"
              "hard"
              "timeo=150"
              "retrans=3"
            ];
          };
          "/mnt/projects" = {
            device = "${config.infra.clients.unas-pro.ip}:/var/nfs/shared/Projects";
            fsType = "nfs";
            options = [
              "x-systemd.requires=network-addresses-br-storage.service" # see /mnt/media
              "x-systemd.after=network-addresses-br-storage.service"
              "x-systemd.automount"
              "noauto"
              "nfsvers=3"
              "rsize=131072"
              "wsize=131072"
              "nconnect=16" # shared transport w/ /mnt/media — must match (see above)
              "noatime"
              "nodiratime"
              "actimeo=10"
              "hard"
              "timeo=150"
              "retrans=3"
            ];
          };
          # Dedicated WD SN580 (xfs) fscache drive. nofail: the cache is optional — a dead
          # or flapping WD must never block boot; cachefilesd just won't start.
          "/var/cache/fscache" = {
            device = "/dev/disk/by-uuid/f6169592-631e-4792-80e2-e229be882fa1";
            fsType = "xfs";
            options = [
              "nofail"
              "x-systemd.device-timeout=10s"
            ];
          };
        };

        # Local NFS page cache for /mnt/media (see the `fsc` mount option). Backed by a
        # dedicated 2TB WD SN580 (xfs) at /var/cache/fscache — a client-side read cache in
        # front of the HDD UNAS, the L2ARC-equivalent the UNAS Pro can't host.
        services.cachefilesd = {
          enable = true;
          cacheDir = "/var/cache/fscache";
          # Dedicated drive: cachefilesd defaults (cull <7% free, hard-stop <3%) let the
          # cache use nearly the whole 2TB. No pool-sharing watermarks needed.
        };

        # Ship fscache hit/miss counters to Grafana. fscache has no exporter, so parse
        # /proc/fs/fscache/stats into a node-exporter textfile *.prom. Server-only: the
        # node-exporter base config lives in the shared logging.vector aspect, and its
        # listOf options merge across modules, so this appends the textfile collector
        # here without touching desktop.
        services.prometheus.exporters.node = {
          enabledCollectors = [ "textfile" ];
          extraFlags = [ "--collector.textfile.directory=/var/lib/node-exporter-textfile" ];
        };

        systemd = {
          # drop dir for the node-exporter textfile collector (fscache exporter below).
          tmpfiles.rules = [
            "d /var/lib/node-exporter-textfile 0755 root root -"
          ];

          services = {
            nfs-media-readahead = {
              after = [ "mnt-media.mount" ];
              wantedBy = [ "mnt-media.mount" ];
              serviceConfig.Type = "oneshot";
              script = "echo 1024 > /sys/class/bdi/$(${pkgs.util-linux}/bin/findmnt -t nfs -frno MAJ:MIN /mnt/media)/read_ahead_kb";
            };

            # Gate cachefilesd on the cache mount so it never initialises onto the bare
            # mountpoint (boot pool) when the WD is absent (implies Requires + After).
            cachefilesd.unitConfig.RequiresMountsFor = "/var/cache/fscache";

            fscache-textfile = {
              description = "Export fscache stats to the node-exporter textfile collector";
              path = [
                pkgs.gawk
                pkgs.coreutils
              ];
              serviceConfig.Type = "oneshot";
              # Atomic write (tmp + rename) so node-exporter never scrapes a partial file.
              # No-op if cachefiles isn't loaded (stats file absent).
              script = ''
                dir=/var/lib/node-exporter-textfile
                src=/proc/fs/fscache/stats
                [ -r "$src" ] || exit 0
                mkdir -p "$dir"
                tmp=$(mktemp "$dir/.fscache.prom.XXXXXX")
                awk '
                  /^Cookies/{for(i=1;i<=NF;i++){split($i,a,"=");if(a[1]=="n")c=a[2]}}
                  /^DownOps/{for(i=1;i<=NF;i++){split($i,a,"=");if(a[1]=="DL")dl=a[2]}}
                  /^CaRdOps/{for(i=1;i<=NF;i++){split($i,a,"=");if(a[1]=="RD")rd=a[2]}}
                  /^CaWrOps/{for(i=1;i<=NF;i++){split($i,a,"=");if(a[1]=="WR")wr=a[2]}}
                  END{
                    print "# HELP fscache_read_hits_total Reads served from local fscache (cache hits)."
                    print "# TYPE fscache_read_hits_total counter"
                    printf "fscache_read_hits_total %d\n", rd+0
                    print "# HELP fscache_downloads_total Reads fetched from the NFS server (cache misses)."
                    print "# TYPE fscache_downloads_total counter"
                    printf "fscache_downloads_total %d\n", dl+0
                    print "# HELP fscache_writes_total Objects written into local fscache."
                    print "# TYPE fscache_writes_total counter"
                    printf "fscache_writes_total %d\n", wr+0
                    print "# HELP fscache_cookies Active fscache cookies (objects tracked)."
                    print "# TYPE fscache_cookies gauge"
                    printf "fscache_cookies %d\n", c+0
                  }
                ' "$src" > "$tmp"
                chmod 0644 "$tmp"
                mv -f "$tmp" "$dir/fscache.prom"
              '';
            };
          };

          timers.fscache-textfile = {
            wantedBy = [ "timers.target" ];
            timerConfig = {
              OnBootSec = "1m";
              OnUnitActiveSec = "30s";
              AccuracySec = "5s";
            };
          };
        };

        disko.devices = {
          disk.main = {
            device = "/dev/nvme0n1";
            type = "disk";
            content = {
              type = "gpt";
              partitions = {
                ESP = {
                  label = "boot";
                  name = "ESP";
                  size = "1G";
                  type = "EF00";
                  priority = 1;
                  uuid = "62090f96-75bb-4384-b3fe-0f780d905f87";
                  content = {
                    type = "filesystem";
                    format = "vfat";
                    mountpoint = "/boot";
                    mountOptions = [
                      "fmask=0022"
                      "dmask=0022"
                    ];
                  };
                };
                root = {
                  size = "100%";
                  uuid = "01e60713-01a5-452f-be62-08305bbbb9e8";
                  content = {
                    type = "btrfs";
                    extraArgs = [ "-f" ];
                    subvolumes = {
                      "/root" = {
                        mountpoint = "/";
                        mountOptions = [
                          "subvol=root"
                          "compress=zstd"
                        ];
                      };
                      "/home" = {
                        mountpoint = "/home";
                        mountOptions = [
                          "subvol=home"
                          "compress=zstd"
                        ];
                      };
                      "/nix" = {
                        mountpoint = "/nix";
                        mountOptions = [
                          "subvol=nix"
                          "compress=zstd"
                          "noatime"
                        ];
                      };
                      "/swap" = {
                        mountpoint = "/swap";
                        swap.swapfile.size = "16G";
                        mountOptions = [
                          "subvol=swap"
                          "noatime"
                        ];
                      };
                      "/log" = {
                        mountpoint = "/var/log";
                        mountOptions = [
                          "subvol=log"
                          "compress=zstd"
                          "noatime"
                        ];
                      };
                      "/appdata" = {
                        mountpoint = "/var/lib/appdata";
                        mountOptions = [
                          "subvol=appdata"
                          "compress=zstd"
                          "noatime"
                        ];
                      };
                      "/workspace" = {
                        mountpoint = "/var/lib/workspace";
                        mountOptions = [
                          "subvol=workspace"
                          "compress=zstd"
                          "noatime"
                        ];
                      };
                      "/buildcache" = {
                        mountpoint = "/var/lib/buildcache";
                        mountOptions = [
                          "subvol=buildcache"
                          "noatime"
                          "nodatacow" # transient build junk: skip CoW+checksums+compression
                        ];
                      };
                    };
                  };
                };
              };
            };
          };
        };

        # container-specific directories and mounts have been migrated to their respective aspect files

      };
  };
}
