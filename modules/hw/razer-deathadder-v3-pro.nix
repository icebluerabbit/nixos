/*
  Razer DeathAdder V3 Pro — wireless ergonomic gaming mouse; openrazer kernel driver + polychromatic GUI
  https://openrazer.github.io
*/
{ den, ... }:
{
  den.aspects.hw._.razer-deathadder-v3-pro = {
    aspect-name = "hw.razer-deathadder-v3-pro";
    includes = [ den.aspects.hw._.rgb ];
    nixos =
      { pkgs, ... }:
      {
        hardware.openrazer.enable = true;
        hardware.openrazer.devicesOffOnScreensaver = false;
        environment.systemPackages = [ pkgs.polychromatic ];
      };
  };
}
