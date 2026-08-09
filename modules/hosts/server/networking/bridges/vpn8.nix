# br-vpn8 — internal p2p bridge joining vpn-gw ↔ its client for tunnel 8.
# No uplink, no host IP; the two containers own 192.168.8.0/24 and attach via extraVeths.
_: {
  den.aspects.hosts._.server._.networking._.bridges._.vpn8 = {
    aspect-name = "hosts.server.networking.bridges.vpn8";
    nixos = {
      networking.bridges.br-vpn8.interfaces = [ ];
    };
  };
}
