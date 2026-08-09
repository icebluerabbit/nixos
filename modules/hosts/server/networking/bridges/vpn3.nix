# br-vpn3 — internal p2p bridge joining vpn-gw ↔ its client for tunnel 3.
# No uplink, no host IP; the two containers own 192.168.3.0/24 and attach via extraVeths.
_: {
  den.aspects.hosts._.server._.networking._.bridges._.vpn3 = {
    aspect-name = "hosts.server.networking.bridges.vpn3";
    nixos = {
      networking.bridges.br-vpn3.interfaces = [ ];
    };
  };
}
