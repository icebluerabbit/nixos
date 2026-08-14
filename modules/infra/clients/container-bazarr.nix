# modules/infra/clients/container-bazarr.nix
{ den, lib, ... }:
(import ../_mk-client.nix { inherit den lib; }) {
  id = "container-bazarr";
  resourceName = "container_bazarr";
  name = "Bazarr Container";
  # index 60 (0xb4) — media's deterministic index range (vlanId 120 + idx)
  # overflows into other VLANs' byte range past idx ~9, so overflow media
  # containers (flaresolverr, autobrr, cross-seed) jump to a high free byte.
  mac = "62:f5:a4:5b:20:b4";
  vlan = "media";
  addressKey = "bazarr";
  ports.default = 6767;
  proxy.port = "default";
}
