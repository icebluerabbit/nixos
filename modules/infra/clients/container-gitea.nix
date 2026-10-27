# modules/infra/clients/container-gitea.nix
{ den, lib, ... }:
let
  client = (import ../_mk-client.nix { inherit lib; }) {
    id = "container-gitea";
    resourceName = "container_gitea";
    name = "Gitea Container";
    mac = "62:f5:a4:5b:20:73";
    vlan = "apps";
    kind = "virtual";
    ports = {
      ssh = 2222;
      web = 3000;
    };
    proxy.port = "web";
  };
in
{
  den.aspects.infra._.clients._.container-gitea = {
    aspect-name = "infra.clients.container-gitea";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
      den.aspects.infra._.networks._.apps
    ];

    os = { config, ... }: client.mkOs config.infra.vlans.apps.staticAddresses.gitea;
    terranix = { config, ... }: client.mkTerranix config.infra.vlans.apps.staticAddresses.gitea;
  };
}
