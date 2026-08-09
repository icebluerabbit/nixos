# platform — host CPU architecture and platform definition for desktop host
_: {
  den.aspects.hosts._.desktop._.platform = {
    aspect-name = "hosts.desktop.platform";
    nixos =
      { lib, ... }:
      {
        nixpkgs.hostPlatform = lib.mkDefault "x86_64-linux";
      };
  };
}
