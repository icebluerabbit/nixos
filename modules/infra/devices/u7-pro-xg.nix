# home network
{ den, lib, ... }:
let
  device = (import ../_mk-device.nix { inherit lib; }) {
    id = "u7-pro-xg";
    resourceName = "ap_u7";
    name = "U7 Pro XG";
    mac = "90:41:b2:ce:6e:f4";
    ip = "10.100.134.21";
    # Uplink: USW Pro HD 24 PoE port 1.
    portProfile = "ap-trunk";
    extra = {
      switch_vlan_enabled = false;
      radio_table = [
        {
          channel = "auto";
          name = "wifi0";
          radio = "ng";
        }
        {
          channel = "auto";
          name = "wifi1";
          radio = "na";
        }
        {
          channel = "auto";
          name = "wifi2";
          radio = "6e";
        }
      ];
    };
  };
in
{
  den.aspects.infra._.devices._.u7-pro-xg = {
    aspect-name = "infra.devices.u7-pro-xg";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
      den.aspects.infra._.ports._.ap-trunk
    ];

    os = _: device.mkOs;
    terranix = { config, ... }: device.mkTerranix config.infra.ports;
  };
}
