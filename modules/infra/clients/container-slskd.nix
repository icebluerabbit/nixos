# modules/infra/clients/container-slskd.nix
{ den, lib, ... }:
let
  client = (import ../_mk-client.nix { inherit lib; }) {
    id = "container-slskd";
    resourceName = "container_slskd";
    name = "slskd Container";
    mac = "62:f5:a4:5b:20:88";
    vlan = "download";
    kind = "virtual";
    ports.web = 5030;
    # managarr-slskd, the sidecar serving this service's records to managarr.
    # 9910 on every managarr-* sidecar: each container has its own IP, so one
    # number carries across all of them with no per-service arithmetic.
    ports.api = 9910;
    proxy.port = "web";
  };
in
{
  den.aspects.infra._.clients._.container-slskd = {
    aspect-name = "infra.clients.container-slskd";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
      den.aspects.infra._.networks._.download
    ];

    os = { config, ... }: client.mkOs config.infra.vlans.download.staticAddresses.slskd;
    terranix = { config, ... }: client.mkTerranix config.infra.vlans.download.staticAddresses.slskd;
  };
}
