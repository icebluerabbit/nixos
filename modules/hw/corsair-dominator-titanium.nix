/*
  corsair-dominator-titanium — DDR5 RGB RAM configuration and boot-time OpenRGB control
  https://gitlab.com/CalcProgrammer1/OpenRGB
*/
{ den, ... }:
{
  den.aspects.hw._.corsair-dominator-titanium = {
    aspect-name = "hw.corsair-dominator-titanium";
    includes = [
      den.aspects.hw._.rgb
      den.aspects.hw._.mem
      den.aspects.hw._.zram
    ];
    nixos =
      { pkgs, lib, ... }:
      let
        dimmCount = 4;
      in
      {
        boot.kernelModules = [ "i2c-dev" ];

        systemd.services.corsair-dominator-rgb = {
          wantedBy = [ "multi-user.target" ];
          after = [ "openrgb.service" ];
          requires = [ "openrgb.service" ];
          serviceConfig = {
            Type = "oneshot";
            RemainAfterExit = true;
            # OpenRGB can't read a model string off Corsair DDR5, so it guesses one
            # per SMBus address: the four sticks come up under three different (wrong)
            # names, two of them identical. --by-type matches on "Type: DRAM" and
            # drives them by index instead — the names are unusable as selectors,
            # and --expect holds out until all ${toString dimmCount} have enumerated.
            #
            # openrgb.service is "started" once its socket is up, but I2C device
            # enumeration takes a few more seconds, hence --retries.
            ExecStart = lib.escapeShellArgs [
              (lib.getExe pkgs.homelab.openrgb-init)
              "--openrgb"
              (lib.getExe pkgs.openrgb)
              "--by-type"
              "DRAM"
              "--expect"
              (toString dimmCount)
              "--retries"
              "30"
              "--set"
              ":ffffff"
            ];
          };
        };
      };
  };
}
