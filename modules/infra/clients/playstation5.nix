# home network
{ den, lib, ... }:
(import ../_mk-client.nix { inherit den lib; }) {
  id = "playstation5";
  resourceName = "playstation5";
  name = "Sony PlayStation 5";
  mac = "b4:0a:d8:2b:e5:68";
  ip = "10.10.80.45";
}
