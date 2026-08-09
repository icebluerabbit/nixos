# home network
{ den, ... }:
let
  name = "USW Flex 2.5G 8";
  mac = "94:2a:6f:4e:cc:ee";
  ip = "10.100.218.255";
  type = "dhcp";
in
{
  den.aspects.infra._.devices._.usw-flex-2-5g-8 = {
    aspect-name = "infra.devices.usw-flex-2-5g-8";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
    ];

    os = _: {
      infra.devices.usw-flex-2-5g-8 = {
        inherit
          name
          mac
          ip
          type
          ;
      };
    };

    terranix = {
      resource.unifi_device.switch_flex = {
        allow_adoption = true;
        config_network = {
          bonding_enabled = false;
          inherit ip type;
        };
        disabled = false;
        flowctrl_enabled = false;
        forget_on_destroy = true;
        jumboframe_enabled = false;
        lcm_brightness_override = false;
        lcm_idle_timeout_override = false;
        inherit mac name;
        site = "default";
        outlet_enabled = false;
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

      import = [
        {
          to = "unifi_device.switch_flex";
          id = mac;
        }
      ];
    };
  };
}
