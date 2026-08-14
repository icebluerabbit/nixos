# qui — modern qBittorrent WebUI (replaces flood)
{ den, lib, ... }:
(import ../_mk-client.nix { inherit den lib; }) {
  id = "container-qui";
  resourceName = "container_qui";
  name = "qui Container";
  mac = "62:f5:a4:5b:20:77";
  vlan = "apps";
  addressKey = "qui";
  ports.default = 7476;
  proxy.port = "default";
}
