# amd-epyc-4545p — hardware profile and CPU optimizations for AMD EPYC 4545P processor
_: {
  den.aspects.hw._.amd-epyc-4545p = {
    aspect-name = "hw.amd-epyc-4545p";
    nixos =
      { config, lib, ... }:
      {
        hardware.cpu.amd.updateMicrocode = lib.mkDefault config.hardware.enableRedistributableFirmware;
        boot.kernelModules = [
          "k10temp"
          "kvm-amd"
        ];
        powerManagement.cpuFreqGovernor = "performance";
        services.irqbalance.enable = true;
      };
  };
}
