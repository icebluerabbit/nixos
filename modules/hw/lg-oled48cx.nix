/*
  lg-oled48cx — LG OLED48CX monitor integration using ddcutil for DDC/CI brightness/input control
  https://github.com/rockowitz/ddcutil
*/
_: {
  den.aspects.hw._.lg-oled48cx = {
    aspect-name = "hw.lg-oled48cx";
    nixos =
      { pkgs, ... }:
      {
        environment.systemPackages = [ pkgs.ddcutil ];
        services.udev.packages = [ pkgs.ddcutil ];
        boot.kernelModules = [ "i2c-dev" ];
      };

    provides.to-users.homeManager = _: {
      # Keyed on monitor make/model/serial, not connector name: this box has two
      # GPUs (RTX 4090 + AMD iGPU) each with one HDMI port, and DRM numbers HDMI
      # connectors in driver probe order — so the monitor races between HDMI-A-1
      # and HDMI-A-2 across reboots. Matching by identity pins scale to the panel.
      programs.niri.settings.outputs."LG Electronics LG TV SSCR2 0x01010101" = {
        mode = {
          width = 3840;
          height = 2160;
          refresh = 119.88;
        };
        scale = 1.5;
      };
    };
  };
}
