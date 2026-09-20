# home network
{ den, lib, ... }:
let
  client = (import ../_mk-client.nix { inherit lib; }) {
    id = "container-qbittorrent";
    resourceName = "container_qbittorrent";
    name = "qBittorrent Container";
    mac = "62:f5:a4:5b:20:85";
    vlan = "download";
    kind = "virtual";
    ports = {
      exporter = 8090;
      webui = 8080;
    };
    # managarr-qbittorrent, the sidecar serving this service's records to managarr.
    # 9910 on every managarr-* sidecar: each container has its own IP, so one
    # number carries across all of them with no per-service arithmetic.
    ports.api = 9910;
    proxy.port = "webui";
  };
in
{
  den.aspects.infra._.clients._.container-qbittorrent = {
    aspect-name = "infra.clients.container-qbittorrent";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
      den.aspects.infra._.networks._.download
    ];

    os = { config, ... }: client.mkOs config.infra.vlans.download.staticAddresses.qbittorrent;
    terranix =
      { config, ... }: client.mkTerranix config.infra.vlans.download.staticAddresses.qbittorrent;
  };
}
