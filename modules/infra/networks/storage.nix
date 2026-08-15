# modules/infra/networks/storage.nix
{ den, lib, ... }:
let
  network = (import ../_mk-network.nix { inherit lib; }) {
    id = "storage";
    resourceName = "storage";
    displayName = "STORAGE";
    vlanId = 50;
    subnet = "10.10.50.0";
    domain = "storage.internal";
    staticAddresses = {
      gateway = "10.10.50.1";
      server = "10.10.50.2";
      unas-pro = "10.10.50.222";
    };
  };
in
{
  den.aspects.infra._.networks._.storage = {
    aspect-name = "infra.networks.storage";
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
