# modules/infra/clients/container-jellyfin.nix
{ den, lib, ... }:
let
  client = (import ../_mk-client.nix { inherit lib; }) {
    id = "container-jellyfin";
    resourceName = "container_jellyfin";
    name = "Jellyfin Container";
    mac = "62:f5:a4:5b:20:7f";
    vlan = "media";
    kind = "virtual";
    ports.http = 8096;
    proxy.port = "http";
  };
in
{
  den.aspects.infra._.clients._.container-jellyfin = {
    aspect-name = "infra.clients.container-jellyfin";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
      den.aspects.infra._.networks._.media
    ];

    os = { config, ... }: client.mkOs config.infra.vlans.media.staticAddresses.jellyfin;
    terranix = { config, ... }: client.mkTerranix config.infra.vlans.media.staticAddresses.jellyfin;
  };
}
