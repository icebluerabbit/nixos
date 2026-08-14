# modules/infra/clients/container-lldap.nix
{ den, lib, ... }:
(import ../_mk-client.nix { inherit den lib; }) {
  id = "container-lldap";
  resourceName = "container_lldap";
  name = "LLDAP Container";
  mac = "62:f5:a4:5b:20:92";
  vlan = "services";
  addressKey = "lldap";
  ports = {
    http = 17170;
    ldap = 3890;
  };
  proxy.port = "http";
}
