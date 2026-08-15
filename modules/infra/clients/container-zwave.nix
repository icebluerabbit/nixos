# modules/infra/clients/container-zwave.nix
{ den, lib, ... }:
let
  client = (import ../_mk-client.nix { inherit lib; }) {
    id = "container-zwave";
    resourceName = "container_zwave";
    name = "Z-Wave Container";
    mac = "62:f5:a4:5b:20:21";
    vlan = "iot";
    kind = "virtual";
    ports = {
      ui = 8091;
      ws = 3000;
    };
    proxy.port = "ui";
  };
in
{
  den.aspects.infra._.clients._.container-zwave = {
    aspect-name = "infra.clients.container-zwave";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
      den.aspects.infra._.networks._.iot
    ];

    os = { config, ... }: client.mkOs config.infra.vlans.iot.staticAddresses.zwave;
    terranix = { config, ... }: client.mkTerranix config.infra.vlans.iot.staticAddresses.zwave;
  };
}
