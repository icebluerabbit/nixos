# home network
{ den, lib, ... }:
(import ../_mk-client.nix { inherit den lib; }) {
  id = "broadlink-remote";
  resourceName = "broadlink_remote";
  name = "BroadLink Remote";
  mac = "34:8e:89:2d:c1:19";
  ip = "10.10.30.76";
}
