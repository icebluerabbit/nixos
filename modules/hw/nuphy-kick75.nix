/*
  nuphy-kick75 — NuPhy Kick75 keyboard configuration with udev rules for VIA interface
  https://github.com/the-via/releases
*/
{ den, ... }:
{
  den.aspects.hw._.nuphy-kick75 = {
    aspect-name = "hw.nuphy-kick75";
    includes = [ den.aspects.hw._.rgb ];
    nixos =
      { pkgs, ... }:
      {
        environment.systemPackages = [ pkgs.via ];
        services.udev.extraRules = ''
          # Kick75 IO (normal mode)
          KERNEL=="hidraw*", SUBSYSTEM=="hidraw", ATTRS{idVendor}=="19f5", ATTRS{idProduct}=="1026", TAG+="uaccess"
          # Kick75 Upgrader (bootloader mode)
          KERNEL=="hidraw*", SUBSYSTEM=="hidraw", ATTRS{idVendor}=="19f5", ATTRS{idProduct}=="0720", TAG+="uaccess"
        '';
      };
  };
}
