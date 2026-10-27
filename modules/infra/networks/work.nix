# modules/infra/networks/work.nix
{ den, lib, ... }:
let
  network = (import ../_mk-network.nix { inherit lib; }) {
    id = "work";
    resourceName = "work";
    displayName = "WORK";
    vlanId = 70;
    subnet = "10.10.70.0";
    domain = "work.internal";
    isolated = true; # Isolated from other local VLANs
    staticAddresses = {
      gateway = "10.10.70.1";
      server = "10.10.70.2";
    };
  };
in
{
  den.aspects.infra._.networks._.work = {
    aspect-name = "infra.networks.work";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
    ];

    os =
      _:
      network.mkOs [
        "1.1.1.1"
        "8.8.8.8"
      ];
    terranix =
      _:
      network.mkTerranix [
        "1.1.1.1"
        "8.8.8.8"
      ];
  };
}
