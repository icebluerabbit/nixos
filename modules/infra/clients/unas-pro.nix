# home network
{ den, lib, ... }:
let
  client = (import ../_mk-client.nix { inherit lib; }) {
    id = "unas-pro";
    resourceName = "unas_pro";
    name = "UNAS-Pro";
    mac = "6c:63:f8:10:41:a2";
    vlan = "storage";
    kind = "physical";
    portProfile = "storage-unas-pro";
  };
in
{
  den.aspects.infra._.clients._.unas-pro = {
    aspect-name = "infra.clients.unas-pro";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
      den.aspects.infra._.networks._.storage
      den.aspects.infra._.ports._.storage-unas-pro
    ];

    os = { config, ... }: client.mkOs config.infra.vlans.storage.staticAddresses.unas-pro;
    terranix = { config, ... }: client.mkTerranix config.infra.vlans.storage.staticAddresses.unas-pro;
  };
}
