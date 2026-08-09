# home network
{ den, ... }:
{
  den.aspects.infra._.networks._.default = {
    aspect-name = "infra.networks.default";
    includes = [ den.aspects.infra._.providers._.unifi ];

    terranix = {
      resource.unifi_network.default = {
        auto_scale = false;
        dhcp_server = {
          boot = {
            enabled = false;
          };
          conflict_checking = false;
          dns_enabled = true;
          dns_servers = [ "10.100.10.1" ];
          enabled = true;
          gateway_enabled = false;
          leasetime = 86400;
          ntp_enabled = false;
          start = "10.100.0.46";
          stop = "10.100.255.254";
          time_offset_enabled = false;
          wins = {
            enabled = false;
          };
        };
        dhcp_guarding = {
          enabled = false;
        };
        dhcp_relay = {
          enabled = false;
        };
        domain_name = "localdomain";
        enabled = true;
        gateway_type = "default";
        igmp_snooping = false;
        internet_access = true;
        ipv6_client_address_assignment = "slaac";
        ipv6_interface_type = "none";
        ipv6_pd_auto_prefixid_enabled = false;
        ipv6_pd_start = "::2";
        ipv6_pd_stop = "::7d1";
        ipv6_ra = true;
        ipv6_ra_preferred_lifetime = 14400;
        ipv6_ra_priority = "high";
        lte_lan = false;
        multicast_dns = true;
        name = "Default";
        network_isolation = false;
        setting_preference = "manual";
        subnet = "10.100.10.1/16";
        third_party_gateway = false;
        site = "default";
      };

      import = [
        {
          to = "unifi_network.default";
          id = "Default";
        }
      ];
    };
  };
}
