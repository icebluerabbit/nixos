# home network
# IoT SSID: legacy devices here cannot do WPA3 or protected management frames,
# and several are 2.4GHz-only.
{ den, lib, ... }:
let
  wlan = (import ../_mk-wlan.nix { inherit lib; }) {
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
  };
in
{
  den.aspects.infra._.wlans._.derp = {
    aspect-name = "infra.wlans.derp";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
      den.aspects.infra._.networks._.iot
    ];

    os = _: wlan.mkOs;
    terranix = _: wlan.mkTerranix;
  };
}
