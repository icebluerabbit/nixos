# modules/infra/clients/container-tuwunel.nix
{ den, lib, ... }:
let
  client = (import ../_mk-client.nix { inherit lib; }) {
    id = "container-tuwunel";
    resourceName = "container_tuwunel";
    name = "Tuwunel Container";
    mac = "62:f5:a4:5b:20:91";
    vlan = "services";
    kind = "virtual";
    ports.default = 6167;
    proxy.port = "default";
  };
in
{
  den.aspects.infra._.clients._.container-tuwunel = {
    aspect-name = "infra.clients.container-tuwunel";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
      den.aspects.infra._.networks._.services
    ];

    os = { config, ... }: client.mkOs config.infra.vlans.services.staticAddresses.tuwunel;
    terranix = { config, ... }: client.mkTerranix config.infra.vlans.services.staticAddresses.tuwunel;
  };
}
