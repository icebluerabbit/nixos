# authelia — forward authentication service
{ den, lib, ... }:
(import ../_mk-client.nix { inherit den lib; }) {
  id = "container-authelia";
  resourceName = "container_authelia";
  name = "Authelia Container";
  mac = "62:f5:a4:5b:20:8f";
  vlan = "services";
  addressKey = "authelia";
  ports = {
    default = 9091;
    metrics = 9959;
  };
  proxy.port = "default";
}
