# modules/infra/clients/container-openchamber.nix
{ den, lib, ... }:
let
  client = (import ../_mk-client.nix { inherit lib; }) {
    id = "container-openchamber";
    resourceName = "container_openchamber";
    name = "OpenChamber Container";
    mac = "62:f5:a4:5b:20:76";
    vlan = "apps";
    kind = "virtual";
    ports.default = 3000;
    proxy.port = "default";
  };
in
{
  den.aspects.infra._.clients._.container-openchamber = {
    aspect-name = "infra.clients.container-openchamber";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
      den.aspects.infra._.networks._.apps
    ];

    os = { config, ... }: client.mkOs config.infra.vlans.apps.staticAddresses.openchamber;
    terranix = { config, ... }: client.mkTerranix config.infra.vlans.apps.staticAddresses.openchamber;
  };
}
