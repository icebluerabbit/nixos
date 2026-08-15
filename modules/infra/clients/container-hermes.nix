# modules/infra/clients/container-hermes.nix
{ den, lib, ... }:
let
  client = (import ../_mk-client.nix { inherit lib; }) {
    id = "container-hermes";
    resourceName = "container_hermes";
    name = "Hermes Agent Container";
    mac = "62:f5:a4:5b:20:99";
    vlan = "services";
    kind = "virtual";
    ports.default = 8642;
    proxy.port = "default";
  };
in
{
  den.aspects.infra._.clients._.container-hermes = {
    aspect-name = "infra.clients.container-hermes";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
      den.aspects.infra._.networks._.services
    ];

    os = { config, ... }: client.mkOs config.infra.vlans.services.staticAddresses.hermes;
    terranix = { config, ... }: client.mkTerranix config.infra.vlans.services.staticAddresses.hermes;
  };
}
