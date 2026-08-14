# modules/infra/clients/container-soularr.nix
{ den, lib, ... }:
(import ../_mk-client.nix { inherit den lib; }) {
  id = "container-soularr";
  resourceName = "container_soularr";
  name = "Soularr Container";
  mac = "62:f5:a4:5b:20:89"; # globally-unique last byte (:87 beets, :88 slskd)
  vlan = "media";
  addressKey = "soularr";
  ports.webui = 8265;
  proxy.port = "webui";
}
