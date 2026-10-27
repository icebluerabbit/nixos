# modules/infra/clients/container-calibre-web.nix
{ den, lib, ... }:
let
  client = (import ../_mk-client.nix { inherit lib; }) {
    id = "container-calibre-web";
    resourceName = "container_calibre_web";
    name = "Calibre-Web Container";
    mac = "62:f5:a4:5b:20:86";
    vlan = "media";
    kind = "virtual";
    ports.default = 8083;
    proxy.port = "default";
  };
in
{
  den.aspects.infra._.clients._.container-calibre-web = {
    aspect-name = "infra.clients.container-calibre-web";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
      den.aspects.infra._.networks._.media
    ];

    os = { config, ... }: client.mkOs config.infra.vlans.media.staticAddresses.calibre-web;
    terranix = { config, ... }: client.mkTerranix config.infra.vlans.media.staticAddresses.calibre-web;
  };
}
