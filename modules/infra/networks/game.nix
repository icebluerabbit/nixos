# modules/infra/networks/game.nix
{ den, lib, ... }:
let
  network = (import ../_mk-network.nix { inherit lib; }) {
    id = "game";
    resourceName = "game";
    displayName = "GAME";
    vlanId = 80;
    subnet = "10.10.80.0";
    domain = "game.internal";
    isolated = true; # Isolated from other local VLANs
    staticAddresses = {
      gateway = "10.10.80.1";
      server = "10.10.80.2";
    };
  };
in
{
  den.aspects.infra._.networks._.game = {
    aspect-name = "infra.networks.game";
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
