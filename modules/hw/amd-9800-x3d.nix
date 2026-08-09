# amd-9800-x3d — hardware profile and CPU optimizations for AMD Ryzen 7 9800X3D processor
_: {
  den.aspects.hw._.amd-9800-x3d = {
    aspect-name = "hw.amd-9800-x3d";
    nixos =
      { lib, config, ... }:
      {
        hardware.cpu.amd.updateMicrocode = lib.mkDefault config.hardware.enableRedistributableFirmware;
        boot = {
          kernelParams = [ "amd_pstate=active" ];
          kernelModules = [
            "k10temp"
            "kvm-amd"
          ];
          kernel.sysctl = {
            "kernel.nmi_watchdog" = 0;
            "kernel.sched_autogroup_enabled" = 0;
          };
        };
        powerManagement.cpuFreqGovernor = "performance";
        services.irqbalance.enable = true;
      };
  };
}
