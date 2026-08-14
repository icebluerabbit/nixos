# modules/infra/clients/container-code-server.nix
{ den, lib, ... }:
(import ../_mk-client.nix { inherit den lib; }) {
  id = "container-code-server";
  resourceName = "container_code_server";
  name = "Code Server Container";
  mac = "62:f5:a4:5b:20:6f"; # apps VLAN, index 1 (next_free_mac)
  vlan = "apps";
  addressKey = "code-server";
  ports.web = 3000;
  proxy.port = "web";
}
