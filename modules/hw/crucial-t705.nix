# crucial-t705 — Crucial T705 NVMe SSD configuration, enabling fstrim, smartd monitoring, and NVMe optimization
_: {
  den.aspects.hw._.crucial-t705 = {
    aspect-name = "hw.crucial-t705";
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
