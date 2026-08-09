/*
  networkmanager — NetworkManager service for dynamic network configuration
  https://networkmanager.dev/
*/
_: {
  den.aspects.networking._.networkmanager = {
    aspect-name = "networking.networkmanager";
    nixos = _: {
      networking = {
        networkmanager = {
          dns = "systemd-resolved";
          enable = true;
          unmanaged = [
            "interface-name:ve-*"
            "interface-name:br-*"
          ];
        };
      };
    };
  };
}
