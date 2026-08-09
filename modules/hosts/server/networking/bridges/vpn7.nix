# br-vpn7 — internal p2p bridge joining vpn-gw ↔ its client for tunnel 7.
# No uplink, no host IP; the two containers own 192.168.7.0/24 and attach via extraVeths.
_: {
  den.aspects.hosts._.server._.networking._.bridges._.vpn7 = {
    aspect-name = "hosts.server.networking.bridges.vpn7";
    nixos = {
      networking.bridges.br-vpn7.interfaces = [ ];
    };
  };
}
