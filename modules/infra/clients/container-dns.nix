# home network
{ den, lib, ... }:
(import ../_mk-client.nix { inherit den lib; }) {
  id = "container-dns";
  resourceName = "container_dns";
  name = "DNS Container";
  mac = "62:f5:a4:5b:20:ad";
  vlan = "services";
  addressKey = "dns";
  ports = {
    dns = 53;
    exporter = 9153;
  };
}
