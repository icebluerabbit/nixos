# modules/infra/clients/container-thelounge.nix
{ den, lib, ... }:
let
  client = (import ../_mk-client.nix { inherit lib; }) {
    id = "container-thelounge";
    resourceName = "container_thelounge";
    name = "TheLounge Container";
    mac = "62:f5:a4:5b:20:75";
    vlan = "apps";
    kind = "virtual";
    ports.default = 9000;
    proxy.port = "default";
  };
in
{
  den.aspects.infra._.clients._.container-thelounge = {
    aspect-name = "infra.clients.container-thelounge";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
      den.aspects.infra._.networks._.apps
    ];

    os = { config, ... }: client.mkOs config.infra.vlans.apps.staticAddresses.thelounge;
    terranix = { config, ... }: client.mkTerranix config.infra.vlans.apps.staticAddresses.thelounge;
  };
}
