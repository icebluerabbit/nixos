# br-vpn6 — internal p2p bridge joining vpn-gw ↔ its client for tunnel 6.
# No uplink, no host IP; the two containers own 192.168.6.0/24 and attach via extraVeths.
_: {
  den.aspects.hosts._.server._.networking._.bridges._.vpn6 = {
    aspect-name = "hosts.server.networking.bridges.vpn6";
    nixos = {
      networking.bridges.br-vpn6.interfaces = [ ];
    };
  };
}
