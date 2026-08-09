# br-vpn10 — internal p2p bridge joining vpn-gw ↔ its client for tunnel 10.
# No uplink, no host IP; the two containers own 192.168.10.0/24 and attach via extraVeths.
_: {
  den.aspects.hosts._.server._.networking._.bridges._.vpn10 = {
    aspect-name = "hosts.server.networking.bridges.vpn10";
    nixos = {
      networking.bridges.br-vpn10.interfaces = [ ];
    };
  };
}
