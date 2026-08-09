# home network
{ den, ... }:
let
  name = "USW Pro HD 24 PoE";
  mac = "a8:9c:6c:04:71:dc";
  ip = "10.100.103.119";
  type = "dhcp";
in
{
  den.aspects.infra._.devices._.usw-pro-hd-24-poe = {
    aspect-name = "infra.devices.usw-pro-hd-24-poe";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
    ];

    os = _: {
      infra.devices.usw-pro-hd-24-poe = {
        inherit
          name
          mac
          ip
          type
          ;
      };
    };

    terranix = {
      resource.unifi_device.switch_pro = {
        allow_adoption = true;
        config_network = {
          bonding_enabled = false;
          inherit ip type;
        };
        disabled = false;
        flowctrl_enabled = false;
        forget_on_destroy = true;
        jumboframe_enabled = false;
        lcm_brightness = 100;
        lcm_brightness_override = false;
        lcm_idle_timeout_override = false;
        lcm_night_mode_begins = "22:00";
        lcm_night_mode_ends = "08:00";
        inherit mac name;
        site = "default";
        outlet_enabled = false;
        port_override = [
          {
            index = 1;
            name = "U7 Pro XG AP";
          }
          {
            index = 2;
            name = "Server";
            port_profile_id = "\${unifi_port_profile.mgmt.id}";
          }
          {
            index = 3;
            name = "Server IPMI";
            port_profile_id = "\${unifi_port_profile.mgmt.id}";
          }
          {
            index = 9;
            name = "USW Flex Uplink";
            port_profile_id = "\${unifi_port_profile.living_room.id}";
          }
          {
            index = 25;
            name = "Server Trunk";
            port_profile_id = "\${unifi_port_profile.server_trunk.id}";
            op_mode = "aggregate";
            aggregate_members = [
              25
              26
            ];
            autoneg = true;
            egress_rate_limit_kbps_enabled = false;
            flow_control_enabled = false;
            full_duplex = true;
            isolation = false;
            lldpmed_enabled = true;
            lldpmed_notify_enabled = true;
            port_keepalive_enabled = true;
            port_security_enabled = false;
            stormctrl_bcast_enabled = false;
            stormctrl_mcast_enabled = false;
            stormctrl_ucast_enabled = false;
            stp_port_mode = true;
          }
          {
            index = 28;
            name = "UNAS Pro";
            port_profile_id = "\${unifi_port_profile.storage.id}";
          }
        ];
        stp_priority = 32768;
        stp_version = "rstp";
        switch_vlan_enabled = false;
      };

      import = [
        {
          to = "unifi_device.switch_pro";
          id = mac;
        }
      ];
    };
  };
}
