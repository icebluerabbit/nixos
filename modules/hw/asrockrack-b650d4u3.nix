# asrockrack-b650d4u3 — ASRock Rack B650D4U3-2L2Q/BCM motherboard profile, enabling network modules and IPMI tools
_: {
  den.aspects.hw._.asrockrack-b650d4u3 = {
    aspect-name = "hw.asrockrack-b650d4u3";
    nixos =
      { pkgs, ... }:
      {
        # Early-boot network drivers support
        boot.initrd.availableKernelModules = [
          "bnxt_en"
          "igb"
          "ast"
        ];

        # IPMI kernel modules and tool support
        boot.kernelModules = [
          "ipmi_si"
          "ipmi_devintf"
          "nct6775"
        ];

        environment.systemPackages = with pkgs; [
          ipmitool
        ];

        systemd.services.asrockrack-bmc-manual-fans = {
          description = "Set ASRock Rack BMC Fan Control to Manual Mode";
          wantedBy = [ "multi-user.target" ];
          before = [ "coolercontrold.service" ];
          serviceConfig = {
            Type = "oneshot";
            RemainAfterExit = true;
            ExecStart = "${pkgs.ipmitool}/bin/ipmitool raw 0x3a 0xd0 0x11 0x2 0x2 0x2 0x2 0x2 0x2 0x2 0x2 0x2 0x2 0x2 0x2 0x2 0x2 0x2 0x2";
          };
        };
      };
  };
}
