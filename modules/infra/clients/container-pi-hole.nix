# modules/infra/clients/container-pi-hole.nix
{ den, lib, ... }:
let
  client = (import ../_mk-client.nix { inherit lib; }) {
    id = "container-pi-hole";
    resourceName = "container_pi_hole";
    name = "Pi-hole Container";
    mac = "62:f5:a4:5b:20:90";
    vlan = "services";
    kind = "virtual";
    ports = {
      dns = 53;
      web = 80;
      exporter = 9617;
    };
    proxy.port = "web";
  };
in
{
  den.aspects.infra._.clients._.container-pi-hole = {
    aspect-name = "infra.clients.container-pi-hole";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
      den.aspects.infra._.networks._.services
    ];

    os = { config, ... }: client.mkOs config.infra.vlans.services.staticAddresses.pi-hole;
    terranix = { config, ... }: client.mkTerranix config.infra.vlans.services.staticAddresses.pi-hole;
  };
}
