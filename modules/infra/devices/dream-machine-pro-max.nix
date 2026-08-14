# home network
{ den, lib, ... }:
(import ../_mk-device.nix { inherit den lib; }) {
  id = "dream-machine-pro-max";
  resourceName = "udm_pro_max";
  name = "Dream Machine Pro Max";
  mac = "74:fa:29:50:06:e5";
  extra = {
    switch_vlan_enabled = false;
  };
}
