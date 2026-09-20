# modules/infra/clients/container-managarr.nix
{ den, lib, ... }:
let
  client = (import ../_mk-client.nix { inherit lib; }) {
    id = "container-managarr";
    resourceName = "container_managarr";
    name = "Managarr Container";
    mac = "62:f5:a4:5b:20:bb";
    vlan = "media";
    kind = "virtual";
    ports = {
      # Every event source posts here, one path per sender under /hook/.
      default = 9911;
      # lib/app's metrics endpoint, bound to loopback for the Vector sidecar.
      metrics = 9090;
    };
    # No proxy: the senders are all on internal VLANs and there is no UI to
    # reach. A Caddy vhost would only widen the reach of an endpoint that
    # deliberately has no authentication.
  };
in
{
  den.aspects.infra._.clients._.container-managarr = {
    aspect-name = "infra.clients.container-managarr";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
      den.aspects.infra._.networks._.media
    ];

    os = { config, ... }: client.mkOs config.infra.vlans.media.staticAddresses.managarr;
    terranix = { config, ... }: client.mkTerranix config.infra.vlans.media.staticAddresses.managarr;
  };
}
