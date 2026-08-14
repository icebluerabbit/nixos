# home network
{ den, lib, ... }:
(import ../_mk-device.nix { inherit den lib; }) {
  id = "usw-flex-2-5g-8";
  resourceName = "switch_flex";
  name = "USW Flex 2.5G 8";
  mac = "94:2a:6f:4e:cc:ee";
  ip = "10.100.218.255";
  extra = {
    port_override = [
      {
        index = 1;
        name = "MacBook Pro Work";
        port_profile_id = "\${unifi_port_profile.work.id}";
      }
      {
        index = 2;
        name = "Hue Hub";
        port_profile_id = "\${unifi_port_profile.iot.id}";
      }
      {
        index = 3;
        name = "PlayStation 5";
        port_profile_id = "\${unifi_port_profile.game.id}";
      }
      {
        index = 4;
        name = "Desktop";
        port_profile_id = "\${unifi_port_profile.trust.id}";
      }
      {
        index = 9;
        name = "Uplink to USW Pro HD 24 PoE";
        port_profile_id = "\${unifi_port_profile.living_room.id}";
      }
    ];
    stp_priority = 32768;
    stp_version = "rstp";
    switch_vlan_enabled = false;
  };
}
