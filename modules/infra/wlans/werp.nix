# home network
{ den, lib, ... }:
(import ../_mk-wlan.nix { inherit den lib; }) {
  id = "werp";
  resourceName = "werp";
  ssid = "werp";
  network = "work";
  unifiId = "6a2e1113efa3052c88b38d46";
  wlanBands = [ "5g" ];
  wlanBand = "5g";
}
