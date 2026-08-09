# platform — host CPU architecture and VM variant shared directory rules configuration for server host
_: {
  den.aspects.hosts._.server._.platform = {
    aspect-name = "hosts.server.platform";
    nixos =
      { config, lib, ... }:
      {
        hardware.cpu.amd.updateMicrocode = lib.mkDefault config.hardware.enableRedistributableFirmware;

        nixpkgs.hostPlatform = lib.mkDefault "x86_64-linux";
        system.stateVersion = lib.mkForce "25.11";
        i18n.defaultLocale = "en_US.UTF-8";
      };
  };
}
