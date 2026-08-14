# home network
{ den, lib, ... }:
(import ../_mk-wlan.nix { inherit den lib; }) {
  id = "herp";
  resourceName = "herp";
  ssid = "herp";
  network = "trust";
  unifiId = "6a28ce03efa3052c88b2e4f9";
  wlanBands = [ "5g" ];
  ignoreWlanBandChanges = true;
}
