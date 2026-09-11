# modules/infra/clients/container-stashtrap.nix
{ den, lib, ... }:
let
  client = (import ../_mk-client.nix { inherit lib; }) {
    id = "container-stashtrap";
    resourceName = "container_stashtrap";
    name = "Stashtrap Container";
    mac = "62:f5:a4:5b:20:ba";
    vlan = "media";
    kind = "virtual";
    ports = {
      # autobrr's external filter posts announces here.
      default = 8080;
      # lib/app's metrics endpoint, bound to loopback for the Vector sidecar.
      metrics = 9090;
    };
    # No proxy: autobrr is the only caller and there is no UI to reach.
  };
in
{
  den.aspects.infra._.clients._.container-stashtrap = {
    aspect-name = "infra.clients.container-stashtrap";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
      den.aspects.infra._.networks._.media
    ];

    os = { config, ... }: client.mkOs config.infra.vlans.media.staticAddresses.stashtrap;
    terranix = { config, ... }: client.mkTerranix config.infra.vlans.media.staticAddresses.stashtrap;
  };
}
