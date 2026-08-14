# home network
{ den, lib, ... }:
(import ../_mk-device.nix { inherit den lib; }) {
  id = "u7-pro-xg";
  resourceName = "ap_u7";
  name = "U7 Pro XG";
  mac = "90:41:b2:ce:6e:f4";
  ip = "10.100.134.21";
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
}
