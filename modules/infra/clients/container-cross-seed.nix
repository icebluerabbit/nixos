# home network
{ den, lib, ... }:
let
  client = (import ../_mk-client.nix { inherit lib; }) {
    id = "container-cross-seed";
    resourceName = "container_cross_seed";
    name = "cross-seed Container";
    mac = "62:f5:a4:5b:20:b3";
    vlan = "media";
    kind = "virtual";
    ports.default = 2468;
    # managarr-crossseed, the read-only sidecar serving cross-seed's decision
    # log. cross-seed's own API has no decisions route and its webhook fires
    # only on a match, so the rejections — most of the record — are reachable no
    # other way. The sidecar runs in this container because that is where the
    # database file is; serving it is what spares every consumer a bind mount.
    #
    # 9910 is the shared managarr-* sidecar port: every container has its own
    # IP, so one number carries across all of them.
    ports.api = 9910;
    proxy.port = "default";
  };
in
{
  den.aspects.infra._.clients._.container-cross-seed = {
    aspect-name = "infra.clients.container-cross-seed";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
      den.aspects.infra._.networks._.media
    ];

    os = { config, ... }: client.mkOs config.infra.vlans.media.staticAddresses.cross-seed;
    terranix = { config, ... }: client.mkTerranix config.infra.vlans.media.staticAddresses.cross-seed;
  };
}
