# home network
{ den, ... }:
let
  name = "Dream Machine Pro Max";
  mac = "74:fa:29:50:06:e5";
  type = "dhcp";
in
{
  den.aspects.infra._.devices._.dream-machine-pro-max = {
    aspect-name = "infra.devices.dream-machine-pro-max";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
    ];

    os = _: {
      infra.devices.dream-machine-pro-max = {
        inherit name mac type;
      };
    };

    terranix = {
      resource.unifi_device.udm_pro_max = {
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
          inherit type;
        };
      };

      import = [
        {
          to = "unifi_device.udm_pro_max";
          id = mac;
        }
      ];
    };
  };
}
