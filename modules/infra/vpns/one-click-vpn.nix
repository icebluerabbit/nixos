# home network
{ den, ... }:
let
  vpn = (import ../_mk-vpn.nix null) {
    id = "one-click-vpn";
    resourceName = "one_click_vpn";
    displayName = "One-Click VPN";
    unifiId = "6a2cdd81efa3052c88b365dc";
    subnet = "192.168.4.1/24";
  };
in
{
  den.aspects.infra._.vpns._.one-click-vpn = {
    aspect-name = "infra.vpns.one-click-vpn";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
      den.aspects.infra._.clients._.container-dns
    ];

    os = _: vpn.mkOs;
    terranix =
      { config, ... }:
      vpn.mkTerranix [ config.infra.clients.container-dns.ip ];
  };
}
