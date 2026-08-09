/*
  bluetooth — kernel Bluetooth stack with power-on-boot and blueman GUI manager
  https://www.bluez.org
*/
_: {
  den.aspects.hw._.bluetooth = {
    aspect-name = "hw.bluetooth";
    nixos =
      { pkgs, ... }:
      {
        hardware.bluetooth.enable = true;
        hardware.bluetooth.powerOnBoot = true;
        services.blueman.enable = true;
        environment.systemPackages = [ pkgs.blueman ];
      };
  };
}
