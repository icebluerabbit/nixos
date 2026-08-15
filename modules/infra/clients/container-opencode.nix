# modules/infra/clients/container-opencode.nix
{ den, lib, ... }:
let
  client = (import ../_mk-client.nix { inherit lib; }) {
    id = "container-opencode";
    resourceName = "container_opencode";
    name = "OpenCode Container";
    mac = "62:f5:a4:5b:20:93";
    vlan = "services";
    kind = "virtual";
    ports.default = 3000;
    proxy.port = "default";
  };
in
{
  den.aspects.infra._.clients._.container-opencode = {
    aspect-name = "infra.clients.container-opencode";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
      den.aspects.infra._.networks._.services
    ];

    os = { config, ... }: client.mkOs config.infra.vlans.services.staticAddresses.opencode;
    terranix = { config, ... }: client.mkTerranix config.infra.vlans.services.staticAddresses.opencode;
  };
}
