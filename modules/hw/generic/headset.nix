/*
  headset — headsetcontrol udev rules and CLI for sidetone, LED, and battery reporting over USB HID
  https://github.com/Sapd/HeadsetControl
*/
_: {
  den.aspects.hw._.headset = {
    aspect-name = "hw.headset";
    nixos =
      { pkgs, ... }:
      {
        environment.systemPackages = [ pkgs.headsetcontrol ];
        services.udev.packages = [ pkgs.headsetcontrol ];
      };
  };
}
