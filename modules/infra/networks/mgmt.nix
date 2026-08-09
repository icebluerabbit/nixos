# modules/infra/networks/mgmt.nix
{ den, ... }:
let
  vlanId = 10;
  subnet = "10.10.10.0";
  subnetPrefix = 24;
  staticAddresses = {
    gateway = "10.10.10.1";
    server = "10.10.10.2";
    server-ipmi = "10.10.10.3";
  };
  domain = "mgmt.internal";

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
  den.aspects.infra._.networks._.mgmt = {
    aspect-name = "infra.networks.mgmt";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
    ];

    os = _: {
      infra.vlans.mgmt = vlanConfig;
    };

    terranix =
      { config, ... }:
      {
        infra.vlans.mgmt = vlanConfig;

        resource.unifi_network.mgmt = {
          name = "MGMT";
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
          network_isolation = false;
          setting_preference = "manual";
          third_party_gateway = false;
          site = "default";
          dhcp_server = {
            boot = {
              enabled = false;
            };
            conflict_checking = true;
            dns_enabled = true;
            dns_servers = [ config.infra.clients.container-dns.ip ];
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
            to = "unifi_network.mgmt";
            id = "MGMT";
          }
        ];
      };
  };
}
