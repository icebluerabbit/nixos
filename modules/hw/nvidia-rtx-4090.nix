/*
  nvidia-rtx-4090 — Nvidia graphics driver configuration, open kernel modules, and environment flags for Wayland
  https://github.com/NVIDIA/open-gpu-kernel-modules
*/
{ den, ... }:
{
  den.aspects.hw._.nvidia-rtx-4090 = {
    aspect-name = "hw.nvidia-rtx-4090";
    includes = [ den.aspects.hw._.graphics ];
    nixos =
      { config, pkgs, ... }:
      {
        services.xserver.videoDrivers = [ "nvidia" ];

        hardware.graphics.extraPackages = with pkgs; [
          nvidia-vaapi-driver
        ];

        hardware.nvidia = {
          modesetting.enable = true;
          powerManagement.enable = true;
          powerManagement.finegrained = false;
          open = true;
          nvidiaSettings = true;
          package = config.boot.kernelPackages.nvidiaPackages.stable;
        };

        boot.kernelParams = [
          "nvidia.NVreg_UsePageAttributeTable=1"
          "nvidia.NVreg_InitializeSystemMemoryAllocations=0"
          "nvidia_drm.fbdev=1"
        ];

        environment.variables = {
          __GLX_VENDOR_LIBRARY_NAME = "nvidia";
          GBM_BACKEND = "nvidia-drm";
          LIBVA_DRIVER_NAME = "nvidia";
          # Fix for Chromium 147+ stream format switches
          NVD_BACKEND = "direct";
          NVD_MAX_SURFACES = "40";
          NVD_ALLOW_MAP_EXPORT = "1";
          NVD_FORCE_DIRECT_IMPORT = "1";
          NVD_DISABLE_BUFFER_SHARING = "0";
          NIXOS_OZONE_WL = "1";
          ELECTRON_OZONE_PLATFORM_HINT = "auto";
        };
      };
  };
}
