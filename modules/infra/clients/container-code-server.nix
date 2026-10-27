# modules/infra/clients/container-code-server.nix
{ den, lib, ... }:
let
  client = (import ../_mk-client.nix { inherit lib; }) {
    id = "container-code-server";
    resourceName = "container_code_server";
    name = "Code Server Container";
    mac = "62:f5:a4:5b:20:6f";
    vlan = "apps";
    kind = "virtual";
    ports.web = 3000;
    proxy.port = "web";
  };
in
{
  den.aspects.infra._.clients._.container-code-server = {
    aspect-name = "infra.clients.container-code-server";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
      den.aspects.infra._.networks._.apps
    ];

    os = { config, ... }: client.mkOs config.infra.vlans.apps.staticAddresses.code-server;
    terranix = { config, ... }: client.mkTerranix config.infra.vlans.apps.staticAddresses.code-server;
  };
}
