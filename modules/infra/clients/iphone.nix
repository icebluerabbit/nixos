# home network
{ den, lib, ... }:
(import ../_mk-client.nix { inherit den lib; }) {
  id = "iphone";
  resourceName = "iphone";
  name = "iPhone";
  mac = "d6:c8:14:8c:ab:3e";
  ip = "10.10.20.60";
}
