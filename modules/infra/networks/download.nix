# modules/infra/networks/download.nix
{ den, lib, ... }:
(import ../_mk-network.nix { inherit den lib; }) {
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
}
