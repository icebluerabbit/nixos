# home network
{ den, ... }:
{
  den.aspects.infra._.wans._.verizon-fios-ont = {
    aspect-name = "infra.wans.verizon-fios-ont";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
    ];
    terranix = {
      resource.unifi_wan.verizon_fios_ont = {
        name = "Verizon Fios ONT";
        type = "dhcp";
        enabled = true;
        load_balance = {
          type = "weighted";
          weight = 99;
          failover_priority = 1;
        };
        setting_preference = "manual";
        report_wan_event = false;
        provider_capabilities = {
          upload_kilobits_per_second = 2543000;
          download_kilobits_per_second = 2365000;
        };
        site = "default";
        mac_override_enabled = false;
        type_v6 = "disabled";
        wan_dslite_remote_host_auto = false;
        ipv6_setting_preference = "manual";
        dhcpv6 = {
          pd_size_auto = false;
          wan_delegation_type = "none";
        };
        dns = {
          ipv6_preference = "auto";
          preference = "auto";
        };
        igmp_proxy = {
          downstream = "none";
          upstream = false;
        };
        vlan = {
          enabled = false;
          id = 0;
        };
      };

      import = [
        {
          to = "unifi_wan.verizon_fios_ont";
          id = "6a20c6afaa20ab3db2a1669b";
        }
      ];
    };
  };
}
