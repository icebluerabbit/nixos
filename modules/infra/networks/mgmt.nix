# modules/infra/networks/mgmt.nix
{ den, lib, ... }:
let
  network = (import ../_mk-network.nix { inherit lib; }) {
    id = "mgmt";
    resourceName = "mgmt";
    displayName = "MGMT";
    vlanId = 10;
    subnet = "10.10.10.0";
    domain = "mgmt.internal";
    staticAddresses = {
      gateway = "10.10.10.1";
      server = "10.10.10.2";
      server-ipmi = "10.10.10.3";
    };
  };
in
{
  den.aspects.infra._.networks._.mgmt = {
    aspect-name = "infra.networks.mgmt";
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
