# modules/infra/clients/container-prometheus.nix
{ den, lib, ... }:
(import ../_mk-client.nix { inherit den lib; }) {
  id = "container-prometheus";
  resourceName = "container_prometheus";
  name = "Prometheus Container";
  mac = "62:f5:a4:5b:20:96";
  vlan = "services";
  addressKey = "prometheus";
  ports.default = 9090;
  proxy.port = "default";
}
