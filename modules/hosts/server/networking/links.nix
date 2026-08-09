# links — physical interface definition and NetworkManager settings
_: {
  den.aspects.hosts._.server._.networking._.links = {
    aspect-name = "hosts.server.networking.links";
    nixos = _: {
      networking = {
        networkmanager.unmanaged = [
          "interface-name:enp5s0f0np0"
          "interface-name:enp5s0f1np1"
          "interface-name:enp9s0"
          "interface-name:vlan-*"
          "interface-name:br-*"
        ];
      };
    };
  };
}
