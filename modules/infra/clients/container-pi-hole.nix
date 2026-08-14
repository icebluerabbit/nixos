# modules/infra/clients/container-pi-hole.nix
{ den, lib, ... }:
(import ../_mk-client.nix { inherit den lib; }) {
  id = "container-pi-hole";
  resourceName = "container_pi_hole";
  name = "Pi-hole Container";
  mac = "62:f5:a4:5b:20:90";
  vlan = "services";
  addressKey = "pi-hole";
  ports = {
    dns = 53;
    web = 80;
  };
  proxy.port = "web";
}
