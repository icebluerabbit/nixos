# br-vpn4 — internal p2p bridge joining vpn-gw ↔ its client for tunnel 4.
# No uplink, no host IP; the two containers own 192.168.4.0/24 and attach via extraVeths.
_: {
  den.aspects.hosts._.server._.networking._.bridges._.vpn4 = {
    aspect-name = "hosts.server.networking.bridges.vpn4";
    nixos = {
      networking.bridges.br-vpn4.interfaces = [ ];
    };
  };
}
