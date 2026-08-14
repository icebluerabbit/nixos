# home network
{ den, lib, ... }:
(import ../_mk-client.nix { inherit den lib; }) {
  id = "hue-hub";
  resourceName = "hue_hub";
  name = "Hue Hub";
  mac = "00:17:88:79:81:e0";
  ip = "10.10.30.33";
}
