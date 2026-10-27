# modules/infra/clients/container-prowlarr.nix
{ den, lib, ... }:
let
  client = (import ../_mk-client.nix { inherit lib; }) {
    id = "container-prowlarr";
    resourceName = "container_prowlarr";
    name = "Prowlarr Container";
    mac = "62:f5:a4:5b:20:7d";
    vlan = "media";
    kind = "virtual";
    ports = {
      default = 9696;
      exporter = 9707;
    };
    # managarr-prowlarr, the sidecar serving this service's records to managarr.
    # 9910 on every managarr-* sidecar: each container has its own IP, so one
    # number carries across all of them with no per-service arithmetic.
    ports.api = 9910;
    proxy.port = "default";
  };
in
{
  den.aspects.infra._.clients._.container-prowlarr = {
    aspect-name = "infra.clients.container-prowlarr";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
      den.aspects.infra._.networks._.media
    ];

    os = { config, ... }: client.mkOs config.infra.vlans.media.staticAddresses.prowlarr;
    terranix = { config, ... }: client.mkTerranix config.infra.vlans.media.staticAddresses.prowlarr;
  };
}
