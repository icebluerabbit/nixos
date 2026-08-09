/*
  asus-rog-strix-x870e-a — ASUS ROG Strix X870E-A Gaming WiFi7 NEO (AM5, X870E) motherboard profile;
  Aura RGB via OpenRGB and Nuvoton Super-I/O hardware monitoring
  https://gitlab.com/CalcProgrammer1/OpenRGB
*/
{ den, ... }:
{
  den.aspects.hw._.asus-rog-strix-x870e-a = {
    aspect-name = "hw.asus-rog-strix-x870e-a";
    includes = [ den.aspects.hw._.rgb ];
    nixos = _: {
      services = {
        hardware = {
          openrgb.motherboard = "amd";

          # Dual rear USB4 ports (ASMedia ASM4242): the thunderbolt driver
          # auto-loads; boltd provides device authorization.
          bolt.enable = true;
        };

        # This board exposes its UEFI BIOS via ESRT capsule-on-disk, so fwupd can
        # flash it (plus TB4/USB4, NVMe, and dbx) straight from LVFS — no BIOS
        # FlashBack USB needed. Update with: fwupdmgr refresh && fwupdmgr update.
        fwupd.enable = true;
      };

      # Board sensors: the Nuvoton NCT6701D Super-I/O and the ASUS EC are not
      # yet supported by mainline (nct6775 tops out at NCT6799 and this board
      # is missing from the asus_ec_sensors DMI table as of kernel 6.16).
      # Both modules are harmless no-ops today and will start exposing
      # VRM/chipset/fan-header sensors as soon as kernel support lands.
      # CPU temps come from k10temp, NVMe from nvme, coolant from the AIO.
      boot.kernelModules = [
        "nct6775"
        "asus_ec_sensors"
      ];
    };
  };
}
