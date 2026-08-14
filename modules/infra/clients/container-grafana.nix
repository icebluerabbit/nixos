# modules/infra/clients/container-grafana.nix
{ den, lib, ... }:
(import ../_mk-client.nix { inherit den lib; }) {
  id = "container-grafana";
  resourceName = "container_grafana";
  name = "Grafana Container";
  mac = "62:f5:a4:5b:20:95";
  vlan = "services";
  addressKey = "grafana";
  ports.default = 3000;
  proxy.port = "default";
}
