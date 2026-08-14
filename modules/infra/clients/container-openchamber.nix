# modules/infra/clients/container-openchamber.nix
{ den, lib, ... }:
(import ../_mk-client.nix { inherit den lib; }) {
  id = "container-openchamber";
  resourceName = "container_openchamber";
  name = "OpenChamber Container";
  mac = "62:f5:a4:5b:20:76";
  vlan = "apps";
  addressKey = "openchamber";
  ports.default = 3000;
  proxy.port = "default";
}
