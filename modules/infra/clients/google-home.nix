# home network
{ den, lib, ... }:
let
  client = (import ../_mk-client.nix { inherit lib; }) {
    id = "google-home";
    resourceName = "google_home";
    name = "Google Home";
    mac = "f4:f5:d8:b4:ff:78";
    vlan = "iot";
    kind = "physical";
  };
in
{
  den.aspects.infra._.clients._.google-home = {
    aspect-name = "infra.clients.google-home";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
      den.aspects.infra._.networks._.iot
    ];

    os = _: client.mkOs "10.10.30.188";
    terranix = _: client.mkTerranix "10.10.30.188";
  };
}
