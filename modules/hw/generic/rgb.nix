/*
  RGB — OpenRGB service and CLI for cross-vendor LED control over USB/I2C/SMBus
  https://gitlab.com/CalcProgrammer1/OpenRGB
*/
_: {
  den.aspects.hw._.rgb = {
    aspect-name = "hw.rgb";
    nixos =
      {
        pkgs,
        lib,
        config,
        ...
      }:
      let
        # Gate OpenRGB's boot detection on CoolerControl's API port: its hwmon/
        # liquidctl scan shares the SMBus, and a transaction landing mid-sequence
        # corrupts OpenRGB's stateful Corsair DRAM device-info read (CRC mismatch
        # -> wrong model / LED count / no Direct). Let CC finish, then detect on a
        # quiet bus. Bounded so a stuck daemon never hangs boot.
        waitForCoolerControl = pkgs.writeShellScript "wait-for-coolercontrol-api" ''
          for _ in $(${pkgs.coreutils}/bin/seq 1 150); do
            ${pkgs.coreutils}/bin/timeout 1 ${pkgs.bash}/bin/bash \
              -c '</dev/tcp/127.0.0.1/11987' 2>/dev/null && exit 0
            ${pkgs.coreutils}/bin/sleep 0.2
          done
          exit 0
        '';
      in
      {
        services.hardware.openrgb.enable = true;
        environment.systemPackages = [ pkgs.openrgb ];

        # Only when the CoolerControl aspect is in this host's config.
        systemd.services.openrgb = lib.mkIf (config.hardware ? coolercontrol) {
          after = [ "coolercontrold.service" ];
          serviceConfig.ExecStartPre = [ waitForCoolerControl ];
        };
      };
  };
}
