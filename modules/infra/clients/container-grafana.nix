# modules/infra/clients/container-grafana.nix
{ den, lib, ... }:
let
  client = (import ../_mk-client.nix { inherit lib; }) {
    id = "container-grafana";
    resourceName = "container_grafana";
    name = "Grafana Container";
    mac = "62:f5:a4:5b:20:95";
    vlan = "services";
    kind = "virtual";
    ports.default = 3000;
    proxy.port = "default";
  };
in
{
  den.aspects.infra._.clients._.container-grafana = {
    aspect-name = "infra.clients.container-grafana";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
      den.aspects.infra._.networks._.services
    ];

    os = { config, ... }: client.mkOs config.infra.vlans.services.staticAddresses.grafana;
    terranix = { config, ... }: client.mkTerranix config.infra.vlans.services.staticAddresses.grafana;
  };
}
