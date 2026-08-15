# modules/infra/clients/container-vpn-gw.nix
{ den, lib, ... }:
let
  client = (import ../_mk-client.nix { inherit lib; }) {
    id = "container-vpn-gw";
    resourceName = "container_vpn_gw";
    name = "VPN Gateway Container";
    mac = "62:f5:a4:5b:20:84";
    vlan = "download";
    kind = "virtual";
    ports = {
      control = 9099;
      nodeExporter = 9100;
      wireguardExporter = 9586;
    };
  };
in
{
  den.aspects.infra._.clients._.container-vpn-gw = {
    aspect-name = "infra.clients.container-vpn-gw";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
      den.aspects.infra._.networks._.download
    ];

    os = { config, ... }: client.mkOs config.infra.vlans.download.staticAddresses.vpnGw;
    terranix = { config, ... }: client.mkTerranix config.infra.vlans.download.staticAddresses.vpnGw;
  };
}
