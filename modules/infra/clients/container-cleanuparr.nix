# modules/infra/clients/container-cleanuparr.nix
{ den, lib, ... }:
let
  client = (import ../_mk-client.nix { inherit lib; }) {
    id = "container-cleanuparr";
    resourceName = "container_cleanuparr";
    name = "Cleanuparr Container";
    mac = "62:f5:a4:5b:20:8b";
    vlan = "media";
    kind = "virtual";
    ports.default = 11011;
    proxy.port = "default";
  };
in
{
  den.aspects.infra._.clients._.container-cleanuparr = {
    aspect-name = "infra.clients.container-cleanuparr";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
      den.aspects.infra._.networks._.media
    ];

    os = { config, ... }: client.mkOs config.infra.vlans.media.staticAddresses.cleanuparr;
    terranix = { config, ... }: client.mkTerranix config.infra.vlans.media.staticAddresses.cleanuparr;
  };
}
