# home network
{ den, lib, ... }:
# IoT SSID: legacy devices here cannot do WPA3 or protected management frames,
# and several are 2.4GHz-only.
(import ../_mk-wlan.nix { inherit den lib; }) {
  id = "derp";
  resourceName = "derp";
  ssid = "derp";
  network = "iot";
  unifiId = "6a28ce28efa3052c88b2e4fd";
  wlanBands = [
    "2g"
    "5g"
  ];
  wlanBand = "both";
  wpa3 = false;
  pmfMode = "disabled";
  bssTransition = false;
  enhancedIot = true;
  no2ghzOui = false;
  macFilterPolicy = "deny";
}
