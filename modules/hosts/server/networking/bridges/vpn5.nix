# br-vpn5 — internal p2p bridge joining vpn-gw ↔ its client for tunnel 5.
# No uplink, no host IP; the two containers own 192.168.5.0/24 and attach via extraVeths.
_: {
  den.aspects.hosts._.server._.networking._.bridges._.vpn5 = {
    aspect-name = "hosts.server.networking.bridges.vpn5";
    nixos = {
      networking.bridges.br-vpn5.interfaces = [ ];
    };
  };
}
