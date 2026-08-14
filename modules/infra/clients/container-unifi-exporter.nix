# modules/infra/clients/container-unifi-exporter.nix
{ den, lib, ... }:
(import ../_mk-client.nix { inherit den lib; }) {
  id = "container-unifi-exporter";
  resourceName = "container_unifi_exporter";
  name = "UniFi Exporter Container";
  mac = "62:f5:a4:5b:20:97";
  vlan = "services";
  addressKey = "unifi-exporter";
  ports = {
    default = 9130;
    syslog = 5140;
  };
}
