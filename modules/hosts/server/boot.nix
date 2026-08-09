# boot — boot loader and kernel module configuration for server host
_: {
  den.aspects.hosts._.server._.boot = {
    aspect-name = "hosts.server.boot";
    nixos =
      { modulesPath, pkgs, ... }:
      {
        imports = [
          (modulesPath + "/installer/scan/not-detected.nix")
        ];

        boot = {
          kernelPackages = pkgs.linuxPackages_latest;

          kernelParams = [
            "pci=realloc"
            "pcie_aspm=off"
          ];

          loader = {
            systemd-boot.enable = true;
            grub.enable = false;
            efi.canTouchEfiVariables = true;
          };

          kernel.sysctl = {
            "net.ipv4.ip_nonlocal_bind" = 1;
            "net.ipv4.conf.all.rp_filter" = 2;
            "net.ipv4.conf.default.rp_filter" = 2;

            # Network and NFS performance tuning for 10GbE / LACP bonded links

            # Max receive buffer size. (Default: 212992 [~208KB])
            # Tuned to 64MB to accommodate the higher Bandwidth Delay Product (BDP) of 10G/20G networks.
            "net.core.rmem_max" = 67108864;

            # Max send buffer size. (Default: 212992 [~208KB])
            # Tuned to 64MB to match the maximum receive buffer limit and allow high-speed uploads.
            "net.core.wmem_max" = 67108864;

            # TCP read buffer limits [min default max]. (Default: "4096 131072 6291456" [~6MB max])
            # Set max to 64MB to let TCP auto-tuning scale window sizes up under heavy loads.
            "net.ipv4.tcp_rmem" = "4096 87380 67108864";

            # TCP write buffer limits [min default max]. (Default: "4096 16384 4194304" [~4MB max])
            # Set max to 64MB to match receive buffers and handle large sequential file writes cleanly.
            "net.ipv4.tcp_wmem" = "4096 65536 67108864";

            # Default queuing discipline. (Default: fq_codel or pfifo_fast)
            # Tuned to Fair Queueing (fq), which is a prerequisite for BBR congestion control.
            "net.core.default_qdisc" = "fq";

            # TCP congestion control algorithm. (Default: cubic)
            # Tuned to BBR, which models network capacity dynamically to maximize throughput and minimize latency.
            "net.ipv4.tcp_congestion_control" = "bbr";

            # Max packets queued in the input queue. (Default: 1000)
            # Pushed up to 100000 to prevent packet drops at the NIC driver layer during high-speed traffic bursts.
            "net.core.netdev_max_backlog" = 100000;

            # Max listen queue depth for socket connections. (Default: 4096)
            # Raised to 8192 to allow more concurrent pending connection requests before packets are ignored.
            "net.core.somaxconn" = 8192;
          };

          # Increase RPC slot table depth to prevent queue bottlenecks on NFS connections. (Default: 256)
          # Tuned to 512 to ensure the client can queue more concurrent requests to saturate
          # the parallel paths enabled by LACP and nconnect.
          extraModprobeConfig = ''
            options sunrpc tcp_slot_table_entries=512
            options sunrpc tcp_max_slot_table_entries=512
          '';

          initrd = {
            availableKernelModules = [
              "xhci_pci"
              "ahci"
              "nvme"
              "usbhid"
              "uas"
              "sd_mod"
            ];
            kernelModules = [ ];
          };
          kernelModules = [ "kvm-amd" ];
          extraModulePackages = [ ];
        };
      };
  };
}
