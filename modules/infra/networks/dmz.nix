# modules/infra/networks/dmz.nix
{ den, lib, ... }:
let
  network = (import ../_mk-network.nix { inherit lib; }) {
    id = "dmz";
    resourceName = "dmz";
    displayName = "DMZ";
    vlanId = 100;
    subnet = "10.10.100.0";
    domain = "dmz.internal";
    staticAddresses = {
      gateway = "10.10.100.1";
      server = "10.10.100.2";
    };
  };
in
{
  den.aspects.infra._.networks._.dmz = {
    aspect-name = "infra.networks.dmz";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
      den.aspects.infra._.clients._.container-dns
    ];

    os =
      { config, ... }:
      network.mkOs [ config.infra.clients.container-dns.ip ];
    terranix =
      { config, ... }:
      network.mkTerranix [ config.infra.clients.container-dns.ip ];
  };
}
