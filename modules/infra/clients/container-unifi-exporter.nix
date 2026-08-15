# modules/infra/clients/container-unifi-exporter.nix
{ den, lib, ... }:
let
  client = (import ../_mk-client.nix { inherit lib; }) {
    id = "container-unifi-exporter";
    resourceName = "container_unifi_exporter";
    name = "UniFi Exporter Container";
    mac = "62:f5:a4:5b:20:97";
    vlan = "services";
    kind = "virtual";
    ports = {
      default = 9130;
      syslog = 5140;
    };
  };
in
{
  den.aspects.infra._.clients._.container-unifi-exporter = {
    aspect-name = "infra.clients.container-unifi-exporter";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
      den.aspects.infra._.networks._.services
    ];

    os = { config, ... }: client.mkOs config.infra.vlans.services.staticAddresses.unifi-exporter;
    terranix =
      { config, ... }: client.mkTerranix config.infra.vlans.services.staticAddresses.unifi-exporter;
  };
}
