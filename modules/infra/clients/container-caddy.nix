# home network
{ den, lib, ... }:
(import ../_mk-client.nix { inherit den lib; }) {
  id = "container-caddy";
  resourceName = "container_caddy";
  name = "Caddy Container";
  mac = "62:f5:a4:5b:20:70";
  vlan = "apps";
  addressKey = "caddy";
  ports = {
    admin = 2019;
    http = 80;
    https = 443;
  };
}
