# modules/infra/clients/container-lldap.nix
{ den, lib, ... }:
let
  client = (import ../_mk-client.nix { inherit lib; }) {
    id = "container-lldap";
    resourceName = "container_lldap";
    name = "LLDAP Container";
    mac = "62:f5:a4:5b:20:92";
    vlan = "services";
    kind = "virtual";
    ports = {
      http = 17170;
      ldap = 3890;
    };
    proxy.port = "http";
  };
in
{
  den.aspects.infra._.clients._.container-lldap = {
    aspect-name = "infra.clients.container-lldap";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
      den.aspects.infra._.networks._.services
    ];

    os = { config, ... }: client.mkOs config.infra.vlans.services.staticAddresses.lldap;
    terranix = { config, ... }: client.mkTerranix config.infra.vlans.services.staticAddresses.lldap;
  };
}
