/*
  corsair-mm700 — Corsair MM700 RGB mousepad color customization via OpenRGB
  https://gitlab.com/CalcProgrammer1/OpenRGB
*/
{ den, ... }:
{
  den.aspects.hw._.corsair-mm700 = {
    aspect-name = "hw.corsair-mm700";
    includes = [ den.aspects.hw._.rgb ];
    nixos =
      { pkgs, lib, ... }:
      {
        systemd.services.corsair-mm700-rgb = {
          wantedBy = [ "multi-user.target" ];
          after = [ "openrgb.service" ];
          requires = [ "openrgb.service" ];
          serviceConfig = {
            Type = "oneshot";
            RemainAfterExit = true;
            ExecStart = pkgs.writeShellScript "corsair-mm700-rgb" ''
              # openrgb.service is "started" once its socket is up, but I2C
              # device enumeration takes a few more seconds — retry until the
              # mousepad appears (or give up after ~30s) so cold boot doesn't race.
              for i in $(seq 1 30); do
                ${lib.getExe pkgs.openrgb} \
                  --device "Corsair MM700" \
                  --mode Direct \
                  --zone 0 \
                  --color ffffff,ffffff,00ff00 && exit 0
                sleep 1
              done
              exit 1
            '';
          };
        };
      };
  };
}
