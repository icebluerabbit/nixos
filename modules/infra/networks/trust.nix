# modules/infra/networks/trust.nix
{ den, lib, ... }:
let
  network = (import ../_mk-network.nix { inherit lib; }) {
    id = "trust";
    resourceName = "trust";
    displayName = "TRUST";
    vlanId = 20;
    subnet = "10.10.20.0";
    domain = "trust.internal";
    staticAddresses = {
      gateway = "10.10.20.1";
      server = "10.10.20.2";
      desktop = "10.10.20.3";
    };
  };
in
{
  den.aspects.infra._.networks._.trust = {
    aspect-name = "infra.networks.trust";
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
