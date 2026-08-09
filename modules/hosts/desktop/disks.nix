# disks — disk partitioning, LUKS encryption, and Btrfs subvolumes layout for desktop host
{ den, ... }:
{
  den.aspects.hosts._.desktop._.disks = {
    aspect-name = "hosts.desktop.disks";
    includes = [
      den.aspects.boot._.disko
      den.aspects.infra._.clients._.unas-pro
    ];

    nixos = { config, ... }: {
      zramSwap = {
        enable = true;
        priority = 100;
      };

      fileSystems = {
        "/mnt/media" = {
          device = "${config.infra.clients.unas-pro.ip}:/var/nfs/shared/Media";
          fsType = "nfs";
          options = [
            "x-systemd.automount"
            "noauto"
            "nfsvers=3"
            "rsize=1048576"
            "wsize=1048576"
            "nconnect=16"
            "noatime"
            "nodiratime"
            "actimeo=3600"
            "hard"
            "timeo=600"
            "retrans=2"
          ];
        };
        "/mnt/data" = {
          device = "${config.infra.clients.unas-pro.ip}:/var/nfs/shared/Data";
          fsType = "nfs";
          options = [
            "x-systemd.automount"
            "noauto"
            "nfsvers=3"
            "rsize=65536"
            "wsize=65536"
            "nconnect=8"
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
            "x-systemd.automount"
            "noauto"
            "nfsvers=3"
            "rsize=131072"
            "wsize=131072"
            "nconnect=8"
            "noatime"
            "nodiratime"
            "actimeo=10"
            "hard"
            "timeo=150"
            "retrans=3"
          ];
        };
      };

      disko.devices = {
        disk.main = {
          # Samsung 9100 PRO 4TB — stable by-id path (position-independent).
          device = "/dev/disk/by-id/nvme-Samsung_SSD_9100_PRO_4TB_S7YANJ0Y311688V";
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
              luks = {
                size = "100%";
                label = "luks";
                priority = 2;
                content = {
                  type = "luks";
                  name = "cryptroot";
                  # Standard disko automation: use a file if present
                  passwordFile = "/tmp/luks.key";
                  extraOpenArgs = [
                    "--allow-discards"
                    "--perf-no_read_workqueue"
                    "--perf-no_write_workqueue"
                  ];
                  content = {
                    type = "btrfs";
                    extraArgs = [
                      "-L"
                      "nixos"
                      "-f"
                    ];
                    subvolumes = {
                      "/root" = {
                        mountpoint = "/";
                        mountOptions = [
                          "subvol=root"
                          "compress=zstd:3"
                          "noatime"
                          "ssd"
                          "discard=async"
                          "space_cache=v2"
                          "commit=120"
                        ];
                      };
                      "/home" = {
                        mountpoint = "/home";
                        mountOptions = [
                          "subvol=home"
                          "compress=zstd:3"
                          "noatime"
                          "ssd"
                          "discard=async"
                          "space_cache=v2"
                          "commit=120"
                          "autodefrag"
                        ];
                      };
                      "/nix" = {
                        mountpoint = "/nix";
                        mountOptions = [
                          "subvol=nix"
                          "compress=zstd:3"
                          "noatime"
                          "ssd"
                          "discard=async"
                          "space_cache=v2"
                          "commit=120"
                        ];
                      };
                      "/persist" = {
                        mountpoint = "/persist";
                        mountOptions = [
                          "subvol=persist"
                          "compress=zstd:3"
                          "noatime"
                          "ssd"
                          "discard=async"
                          "space_cache=v2"
                          "commit=120"
                        ];
                      };
                      "/log" = {
                        mountpoint = "/var/log";
                        mountOptions = [
                          "subvol=log"
                          "compress=zstd:3"
                          "noatime"
                          "ssd"
                          "discard=async"
                          "space_cache=v2"
                          "commit=120"
                        ];
                      };
                      "/swap" = {
                        mountpoint = "/swap";
                        swap.swapfile = {
                          size = "8G";
                          priority = -2;
                        };
                        mountOptions = [
                          "subvol=swap"
                          "noatime"
                          "nodatacow"
                          "nodatasum"
                        ];
                      };
                    };
                  };
                };
              };
            };
          };
        };
      };
    };
  };
}
