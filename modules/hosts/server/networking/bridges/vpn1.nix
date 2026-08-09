# br-vpn1 — internal p2p bridge joining vpn-gw ↔ its client for tunnel 1.
# No uplink, no host IP; the two containers own 192.168.1.0/24 and attach via extraVeths.
_: {
  den.aspects.hosts._.server._.networking._.bridges._.vpn1 = {
    aspect-name = "hosts.server.networking.bridges.vpn1";
    nixos = {
      networking.bridges.br-vpn1.interfaces = [ ];
    };
  };
}
