# home network
{ den, lib, ... }:
let
  wlan = (import ../_mk-wlan.nix { inherit lib; }) {
    id = "herp";
    resourceName = "herp";
    ssid = "herp";
    network = "trust";
    unifiId = "6a28ce03efa3052c88b2e4f9";
    wlanBands = [ "5g" ];
    ignoreWlanBandChanges = true;
  };
in
{
  den.aspects.infra._.wlans._.herp = {
    aspect-name = "infra.wlans.herp";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
      den.aspects.infra._.networks._.trust
    ];

    os = _: wlan.mkOs;
    terranix = _: wlan.mkTerranix;
  };
}
