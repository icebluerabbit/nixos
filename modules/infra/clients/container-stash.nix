# modules/infra/clients/container-stash.nix
{ den, lib, ... }:
let
  client = (import ../_mk-client.nix { inherit lib; }) {
    id = "container-stash";
    resourceName = "container_stash";
    name = "Stash Container";
    mac = "62:f5:a4:5b:20:b9";
    vlan = "media";
    kind = "virtual";
    ports = {
      default = 9999;
    };
    proxy.port = "default";
  };
in
{
  den.aspects.infra._.clients._.container-stash = {
    aspect-name = "infra.clients.container-stash";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
      den.aspects.infra._.networks._.media
    ];

    os = { config, ... }: client.mkOs config.infra.vlans.media.staticAddresses.stash;
    terranix = { config, ... }: client.mkTerranix config.infra.vlans.media.staticAddresses.stash;
  };
}
