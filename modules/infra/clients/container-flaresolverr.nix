# home network
{ den, lib, ... }:
(import ../_mk-client.nix { inherit den lib; }) {
  id = "container-flaresolverr";
  resourceName = "container_flaresolverr";
  name = "FlareSolverr Container";
  mac = "62:f5:a4:5b:20:83";
  vlan = "media";
  addressKey = "flaresolverr";
  ports.default = 8191;
}
