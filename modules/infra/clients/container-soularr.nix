# modules/infra/clients/container-soularr.nix
{ den, lib, ... }:
let
  client = (import ../_mk-client.nix { inherit lib; }) {
    id = "container-soularr";
    resourceName = "container_soularr";
    name = "Soularr Container";
    mac = "62:f5:a4:5b:20:89";
    vlan = "media";
    kind = "virtual";
    ports.webui = 8265;
    # managarr-soularr, the sidecar serving this service's records to managarr.
    # 9910 on every managarr-* sidecar: each container has its own IP, so one
    # number carries across all of them with no per-service arithmetic.
    ports.api = 9910;
    proxy.port = "webui";
  };
in
{
  den.aspects.infra._.clients._.container-soularr = {
    aspect-name = "infra.clients.container-soularr";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
      den.aspects.infra._.networks._.media
    ];

    os = { config, ... }: client.mkOs config.infra.vlans.media.staticAddresses.soularr;
    terranix = { config, ... }: client.mkTerranix config.infra.vlans.media.staticAddresses.soularr;
  };
}
