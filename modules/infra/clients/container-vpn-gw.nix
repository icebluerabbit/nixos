# modules/infra/clients/container-vpn-gw.nix
{ den, lib, ... }:
(import ../_mk-client.nix { inherit den lib; }) {
  id = "container-vpn-gw";
  resourceName = "container_vpn_gw";
  name = "VPN Gateway Container";
  mac = "62:f5:a4:5b:20:84";
  vlan = "download";
  addressKey = "vpnGw";
  ports = {
    control = 9099;
    nodeExporter = 9100;
    wireguardExporter = 9586;
  };
}
