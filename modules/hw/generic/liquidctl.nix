/*
  liquidctl — USB liquid cooler and RGB controller driver; udev rules and CLI for AIO and fan hubs
  https://github.com/liquidctl/liquidctl
*/
_: {
  den.aspects.hw._.liquidctl = {
    aspect-name = "hw.liquidctl";
    nixos =
      { pkgs, ... }:
      {
        environment.systemPackages = [ pkgs.liquidctl ];
        services.udev.packages = [ pkgs.liquidctl ];
      };
  };
}
