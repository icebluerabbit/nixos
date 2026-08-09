# modules/infra/networks/trust.nix
{ den, ... }:
let
  vlanId = 20;
  subnet = "10.10.20.0";
  subnetPrefix = 24;
  staticAddresses = {
    gateway = "10.10.20.1";
    server = "10.10.20.2";
    desktop = "10.10.20.3";
  };
  domain = "trust.internal";

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
  den.aspects.infra._.networks._.trust = {
    aspect-name = "infra.networks.trust";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
    ];

    os = _: {
      infra.vlans.trust = vlanConfig;
    };

    terranix =
      { config, ... }:
      {
        infra.vlans.trust = vlanConfig;

        resource.unifi_network.trust = {
          name = "TRUST";
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
            to = "unifi_network.trust";
            id = "TRUST";
          }
        ];
      };
  };
}
