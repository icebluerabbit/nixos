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
            ExecStart = pkgs.writeShellScript "corsair-dominator-rgb" ''
              # OpenRGB can't read a model string off Corsair DDR5, so it guesses one
              # per SMBus address: the four sticks come up under three different (wrong)
              # names, two of them identical. Match on Type: DRAM and drive them by
              # index instead — the names are unusable as selectors.
              #
              # openrgb.service is "started" once its socket is up, but I2C device
              # enumeration takes a few more seconds — retry until all ${toString dimmCount}
              # DIMMs appear (or give up after ~30s) so cold boot doesn't race.
              for i in $(seq 1 30); do
                dimms=$(${lib.getExe pkgs.openrgb} --list-devices 2>/dev/null | ${pkgs.gawk}/bin/awk '
                  /^[0-9]+:/ { idx = $1; sub(":", "", idx) }
                  /Type:[[:space:]]+DRAM/ { print idx }
                ')
                if [ "$(printf '%s\n' "$dimms" | ${pkgs.coreutils}/bin/wc -w)" -eq ${toString dimmCount} ]; then
                  for d in $dimms; do
                    ${lib.getExe pkgs.openrgb} \
                      --device "$d" \
                      --mode Direct \
                      --color ffffff || exit 1
                  done
                  exit 0
                fi
                sleep 1
              done
              exit 1
            '';
          };
        };
      };
  };
}
