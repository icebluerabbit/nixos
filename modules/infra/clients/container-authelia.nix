# authelia — forward authentication service
{ den, lib, ... }:
let
  client = (import ../_mk-client.nix { inherit lib; }) {
    id = "container-authelia";
    resourceName = "container_authelia";
    name = "Authelia Container";
    mac = "62:f5:a4:5b:20:8f";
    vlan = "services";
    kind = "virtual";
    ports = {
      default = 9091;
      metrics = 9959;
    };
    proxy.port = "default";
  };
in
{
  den.aspects.infra._.clients._.container-authelia = {
    aspect-name = "infra.clients.container-authelia";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
      den.aspects.infra._.networks._.services
    ];

    os = { config, ... }: client.mkOs config.infra.vlans.services.staticAddresses.authelia;
    terranix = { config, ... }: client.mkTerranix config.infra.vlans.services.staticAddresses.authelia;
  };
}
