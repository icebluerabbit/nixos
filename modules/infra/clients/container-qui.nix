# qui — modern qBittorrent WebUI (replaces flood)
{ den, lib, ... }:
let
  client = (import ../_mk-client.nix { inherit lib; }) {
    id = "container-qui";
    resourceName = "container_qui";
    name = "qui Container";
    mac = "62:f5:a4:5b:20:77";
    vlan = "apps";
    kind = "virtual";
    ports.default = 7476;
    proxy.port = "default";
  };
in
{
  den.aspects.infra._.clients._.container-qui = {
    aspect-name = "infra.clients.container-qui";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
      den.aspects.infra._.networks._.apps
    ];

    os = { config, ... }: client.mkOs config.infra.vlans.apps.staticAddresses.qui;
    terranix = { config, ... }: client.mkTerranix config.infra.vlans.apps.staticAddresses.qui;
  };
}
