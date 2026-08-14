# modules/infra/clients/container-slskd.nix
{ den, lib, ... }:
(import ../_mk-client.nix { inherit den lib; }) {
  id = "container-slskd";
  resourceName = "container_slskd";
  name = "slskd Container";
  mac = "62:f5:a4:5b:20:88"; # globally-unique last byte (:87 is beets); MACs must be unique network-wide
  vlan = "download";
  addressKey = "slskd";
  # web UI (LAN). Soulseek inbound rides the VPN tunnel DNAT, not a LAN port.
  ports.web = 5030;
  proxy.port = "web";
}
