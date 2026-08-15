# modules/infra/clients/container-prometheus.nix
{ den, lib, ... }:
let
  client = (import ../_mk-client.nix { inherit lib; }) {
    id = "container-prometheus";
    resourceName = "container_prometheus";
    name = "Prometheus Container";
    mac = "62:f5:a4:5b:20:96";
    vlan = "services";
    kind = "virtual";
    ports.default = 9090;
    proxy.port = "default";
  };
in
{
  den.aspects.infra._.clients._.container-prometheus = {
    aspect-name = "infra.clients.container-prometheus";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
      den.aspects.infra._.networks._.services
    ];

    os = { config, ... }: client.mkOs config.infra.vlans.services.staticAddresses.prometheus;
    terranix =
      { config, ... }: client.mkTerranix config.infra.vlans.services.staticAddresses.prometheus;
  };
}
