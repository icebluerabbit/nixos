# modules/infra/clients/container-bindery.nix
{ den, lib, ... }:
let
  client = (import ../_mk-client.nix { inherit lib; }) {
    id = "container-bindery";
    resourceName = "container_bindery";
    name = "Bindery Container";
    mac = "62:f5:a4:5b:20:b5";
    vlan = "media";
    kind = "virtual";
    ports.default = 8787;
    proxy.port = "default";
  };
in
{
  den.aspects.infra._.clients._.container-bindery = {
    aspect-name = "infra.clients.container-bindery";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
      den.aspects.infra._.networks._.media
    ];

    os = { config, ... }: client.mkOs config.infra.vlans.media.staticAddresses.bindery;
    terranix = { config, ... }: client.mkTerranix config.infra.vlans.media.staticAddresses.bindery;
  };
}
