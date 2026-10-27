# home network
{ den, lib, ... }:
let
  client = (import ../_mk-client.nix { inherit lib; }) {
    id = "container-dns";
    resourceName = "container_dns";
    name = "DNS Container";
    mac = "62:f5:a4:5b:20:ad";
    vlan = "services";
    kind = "virtual";
    ports = {
      dns = 53;
      exporter = 9153;
    };
  };
in
{
  den.aspects.infra._.clients._.container-dns = {
    aspect-name = "infra.clients.container-dns";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
      den.aspects.infra._.networks._.services
    ];

    os = { config, ... }: client.mkOs config.infra.vlans.services.staticAddresses.dns;
    terranix = { config, ... }: client.mkTerranix config.infra.vlans.services.staticAddresses.dns;
  };
}
