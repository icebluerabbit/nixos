# modules/infra/networks/download.nix
{ den, lib, ... }:
let
  network = (import ../_mk-network.nix { inherit lib; }) {
    id = "download";
    resourceName = "download";
    displayName = "DOWNLOAD";
    vlanId = 130;
    subnet = "10.10.130.0";
    domain = "download.internal";
    staticAddresses = {
      gateway = "10.10.130.1";
      server = "10.10.130.2";
      vpnGw = "10.10.130.4";
      qbittorrent = "10.10.130.5";
      slskd = "10.10.130.6";
    };
  };
in
{
  den.aspects.infra._.networks._.download = {
    aspect-name = "infra.networks.download";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
      den.aspects.infra._.clients._.container-dns
    ];

    os =
      { config, ... }:
      network.mkOs [ config.infra.clients.container-dns.ip ];
    terranix =
      { config, ... }:
      network.mkTerranix [ config.infra.clients.container-dns.ip ];
  };
}
