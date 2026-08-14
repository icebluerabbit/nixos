# modules/infra/clients/container-opencode.nix
{ den, lib, ... }:
(import ../_mk-client.nix { inherit den lib; }) {
  id = "container-opencode";
  resourceName = "container_opencode";
  name = "OpenCode Container";
  mac = "62:f5:a4:5b:20:93";
  vlan = "services";
  addressKey = "opencode";
  ports.default = 3000;
  proxy.port = "default";
}
