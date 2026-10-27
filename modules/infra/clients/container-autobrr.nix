# home network
{ den, lib, ... }:
let
  client = (import ../_mk-client.nix { inherit lib; }) {
    id = "container-autobrr";
    resourceName = "container_autobrr";
    name = "Autobrr Container";
    mac = "62:f5:a4:5b:20:b2";
    vlan = "media";
    kind = "virtual";
    ports.default = 7474;
    # managarr-autobrr, the sidecar serving this service's records to managarr.
    # 9910 on every managarr-* sidecar: each container has its own IP, so one
    # number carries across all of them with no per-service arithmetic.
    ports.api = 9910;
    proxy.port = "default";
  };
in
{
  den.aspects.infra._.clients._.container-autobrr = {
    aspect-name = "infra.clients.container-autobrr";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
      den.aspects.infra._.networks._.media
    ];

    os = { config, ... }: client.mkOs config.infra.vlans.media.staticAddresses.autobrr;
    terranix = { config, ... }: client.mkTerranix config.infra.vlans.media.staticAddresses.autobrr;
  };
}
