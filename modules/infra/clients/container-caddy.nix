# home network
{ den, lib, ... }:
let
  client = (import ../_mk-client.nix { inherit lib; }) {
    id = "container-caddy";
    resourceName = "container_caddy";
    name = "Caddy Container";
    mac = "62:f5:a4:5b:20:70";
    vlan = "apps";
    kind = "virtual";
    ports = {
      admin = 2019;
      http = 80;
      https = 443;
    };
  };
in
{
  den.aspects.infra._.clients._.container-caddy = {
    aspect-name = "infra.clients.container-caddy";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
      den.aspects.infra._.networks._.apps
    ];

    os = { config, ... }: client.mkOs config.infra.vlans.apps.staticAddresses.caddy;
    terranix = { config, ... }: client.mkTerranix config.infra.vlans.apps.staticAddresses.caddy;
  };
}
