# br-vpn2 — internal p2p bridge joining vpn-gw ↔ its client for tunnel 2.
# No uplink, no host IP; the two containers own 192.168.2.0/24 and attach via extraVeths.
_: {
  den.aspects.hosts._.server._.networking._.bridges._.vpn2 = {
    aspect-name = "hosts.server.networking.bridges.vpn2";
    nixos = {
      networking.bridges.br-vpn2.interfaces = [ ];
    };
  };
}
