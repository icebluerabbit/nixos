# home network
{ den, ... }:
let
  name = "U7 Pro XG";
  mac = "90:41:b2:ce:6e:f4";
  ip = "10.100.134.21";
  type = "dhcp";
in
{
  den.aspects.infra._.devices._.u7-pro-xg = {
    aspect-name = "infra.devices.u7-pro-xg";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
    ];

    os = _: {
      infra.devices.u7-pro-xg = {
        inherit
          name
          mac
          ip
          type
          ;
      };
    };

    terranix = {
      resource.unifi_device.ap_u7 = {
        inherit mac name;
        allow_adoption = true;
        forget_on_destroy = true;
        site = "default";
        disabled = false;
        flowctrl_enabled = false;
        jumboframe_enabled = false;
        lcm_brightness_override = false;
        lcm_idle_timeout_override = false;
        outlet_enabled = false;
        switch_vlan_enabled = false;
        config_network = {
          bonding_enabled = false;
          inherit ip type;
        };
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

      import = [
        {
          to = "unifi_device.ap_u7";
          id = mac;
        }
      ];
    };
  };
}
