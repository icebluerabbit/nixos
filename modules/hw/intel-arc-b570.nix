# intel-arc-b570 — Intel Arc B570 graphics card aspect, enabling hardware acceleration and compute runtimes
{ den, ... }:
{
  den.aspects.hw._.intel-arc-b570 = {
    aspect-name = "hw.intel-arc-b570";
    includes = [ den.aspects.hw._.graphics ];
    nixos =
      { pkgs, ... }:
      {
        hardware.graphics.extraPackages = with pkgs; [
          intel-media-driver
          vpl-gpu-rt
          intel-compute-runtime
        ];

        boot.initrd.kernelModules = [ "xe" ];

        environment.sessionVariables = {
          LIBVA_DRIVER_NAME = "iHD";
        };
      };
  };
}
