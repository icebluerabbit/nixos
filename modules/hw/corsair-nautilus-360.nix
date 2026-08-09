# corsair-nautilus-360 — Corsair Nautilus 360 RS LCD liquid cooler aspect
{ den, ... }:
let
  # Motherboard UID discovered in logs
  motherboardUid = "00a4da18625f56275c89e2fcd25a83c08c5ad3326452fa7e252fcc8a89c92493";
in
{
  den.aspects.hw._.corsair-nautilus-360 = {
    aspect-name = "hw.corsair-nautilus-360";
    includes = [
      den.aspects.hw._.rgb
      den.aspects.hw._.coolercontrol
    ];
    nixos = _: {
      services.udev.extraRules = ''
        # Corsair Nautilus LCD Cap
        SUBSYSTEM=="usb", ATTR{idVendor}=="1b1c", ATTR{idProduct}=="0c55", MODE="0666", GROUP="users"
      '';

      hardware.coolercontrol.deviceSettings.${motherboardUid} = {
        # Two extra chassis fans
        fan1.speed_fixed = 100;
        fan2.speed_fixed = 100;

        # AIO Fans
        fan3.speed_fixed = 100;
        fan5.speed_fixed = 100;
        fan7.speed_fixed = 100;

        # AIO Pump
        fan4.speed_fixed = 100;
      };
    };
  };
}
