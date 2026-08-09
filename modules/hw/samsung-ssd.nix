/*
  samsung-ssd — Samsung NVMe SSD configuration, enabling fstrim, smartd monitoring, and NVMe optimization
  https://github.com/linux-nvme/nvme-cli
*/
_: {
  den.aspects.hw._.samsung-ssd = {
    aspect-name = "hw.samsung-ssd";
    nixos =
      { pkgs, ... }:
      {
        services = {
          fstrim.enable = true;
          smartd.enable = true;
          udev.extraRules = ''
            ACTION=="add|change", KERNEL=="nvme[0-9]n[0-9]", ATTR{queue/scheduler}="none"
          '';
        };
        environment.systemPackages = [ pkgs.nvme-cli ];
        boot.kernelParams = [ "nvme_core.default_ps_max_latency_us=0" ];
      };
  };
}
