# modules/infra/clients/container-radarr.nix
{ den, lib, ... }:
let
  client = (import ../_mk-client.nix { inherit lib; }) {
    id = "container-radarr";
    resourceName = "container_radarr";
    name = "Radarr Container";
    mac = "62:f5:a4:5b:20:79";
    vlan = "media";
    kind = "virtual";
    ports = {
      default = 7878;
      exporter = 9707;
    };
    proxy.port = "default";
  };
in
{
  den.aspects.infra._.clients._.container-radarr = {
    aspect-name = "infra.clients.container-radarr";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
      den.aspects.infra._.networks._.media
    ];

    os = { config, ... }: client.mkOs config.infra.vlans.media.staticAddresses.radarr;
    terranix = { config, ... }: client.mkTerranix config.infra.vlans.media.staticAddresses.radarr;
  };
}
