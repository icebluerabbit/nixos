# boot — boot loader and kernel module configuration for desktop host
_: {
  den.aspects.hosts._.desktop._.boot = {
    aspect-name = "hosts.desktop.boot";
    nixos =
      { pkgs, ... }:
      {
        boot = {
          kernelPackages = pkgs.linuxPackages_latest;
          supportedFilesystems = [ "exfat" ];
          loader = {
            systemd-boot.enable = true;
            grub.enable = false;
            efi.canTouchEfiVariables = true;
          };
          initrd = {
            systemd.enable = true;
            availableKernelModules = [
              "nvme"
              "ahci"
              "xhci_pci"
              "thunderbolt"
              "usbhid"
              "uas"
              "sd_mod"
            ];
          };
          zfs.forceImportRoot = false;
        };
      };
  };
}
