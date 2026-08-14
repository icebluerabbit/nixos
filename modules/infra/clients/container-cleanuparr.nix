# modules/infra/clients/container-cleanuparr.nix
{ den, lib, ... }:
(import ../_mk-client.nix { inherit den lib; }) {
  id = "container-cleanuparr";
  resourceName = "container_cleanuparr";
  name = "Cleanuparr Container";
  # Not vlanId + index: media's contiguous run is exhausted through :8a, and the
  # last byte is a network-wide space, so this is the lowest globally-free slot.
  mac = "62:f5:a4:5b:20:8b";
  vlan = "media";
  addressKey = "cleanuparr";
  ports.default = 11011;
  proxy.port = "default";
}
