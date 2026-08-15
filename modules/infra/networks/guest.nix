# modules/infra/networks/guest.nix
{ den, lib, ... }:
let
  network = (import ../_mk-network.nix { inherit lib; }) {
    id = "guest";
    resourceName = "guest";
    displayName = "GUEST";
    vlanId = 40;
    subnet = "10.10.40.0";
    domain = "guest.internal";
    isolated = true; # Isolated from other local VLANs
    staticAddresses = {
      gateway = "10.10.40.1";
      server = "10.10.40.2";
    };
  };
in
{
  den.aspects.infra._.networks._.guest = {
    aspect-name = "infra.networks.guest";
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
