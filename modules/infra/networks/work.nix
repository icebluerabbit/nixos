# modules/infra/networks/work.nix
{ den, ... }:
let
  vlanId = 70;
  subnet = "10.10.70.0";
  subnetPrefix = 24;
  staticAddresses = {
    gateway = "10.10.70.1";
    server = "10.10.70.2";
  };
  domain = "work.internal";

  vlanConfig = {
    inherit
      vlanId
      subnet
      subnetPrefix
      domain
      staticAddresses
      ;
  };
in
{
  den.aspects.infra._.networks._.work = {
    aspect-name = "infra.networks.work";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
    ];

    os = _: {
      infra.vlans.work = vlanConfig;
    };

    terranix = _: {
      infra.vlans.work = vlanConfig;

      resource.unifi_network.work = {
        name = "WORK";
        vlan = vlanId;
        subnet = "${staticAddresses.gateway}/${toString subnetPrefix}";
        domain_name = domain;
        auto_scale = false;
        enabled = true;
        gateway_type = "default";
        igmp_snooping = false;
        internet_access = true;
        ipv6_interface_type = "none";
        multicast_dns = true;
        network_isolation = true; # Isolated from other local VLANs
        setting_preference = "manual";
        third_party_gateway = false;
        site = "default";
        dhcp_server = {
          boot = {
            enabled = false;
          };
          conflict_checking = true;
          dns_enabled = true;
          dns_servers = [
            "1.1.1.1"
            "8.8.8.8"
          ];
          enabled = true;
          gateway_enabled = false;
          leasetime = 86400;
          ntp_enabled = false;
          start = "10.10.${toString vlanId}.100";
          stop = "10.10.${toString vlanId}.200";
          time_offset_enabled = false;
          wins = {
            enabled = false;
          };
        };
      };

      import = [
        {
          to = "unifi_network.work";
          id = "WORK";
        }
      ];
    };
  };
}
