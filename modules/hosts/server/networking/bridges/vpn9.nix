# br-vpn9 — internal p2p bridge joining vpn-gw ↔ its client for tunnel 9.
# No uplink, no host IP; the two containers own 192.168.9.0/24 and attach via extraVeths.
_: {
  den.aspects.hosts._.server._.networking._.bridges._.vpn9 = {
    aspect-name = "hosts.server.networking.bridges.vpn9";
    nixos = {
      networking.bridges.br-vpn9.interfaces = [ ];
    };
  };
}
