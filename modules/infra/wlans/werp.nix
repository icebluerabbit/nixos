# home network
{ den, lib, ... }:
let
  wlan = (import ../_mk-wlan.nix { inherit lib; }) {
    id = "werp";
    resourceName = "werp";
    ssid = "werp";
    network = "work";
    unifiId = "6a2e1113efa3052c88b38d46";
    wlanBands = [ "5g" ];
    wlanBand = "5g";
  };
in
{
  den.aspects.infra._.wlans._.werp = {
    aspect-name = "infra.wlans.werp";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
      den.aspects.infra._.networks._.work
    ];

    os = _: wlan.mkOs;
    terranix = _: wlan.mkTerranix;
  };
}
