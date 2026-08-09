# modules/infra/networks/guest.nix
{ den, ... }:
let
  vlanId = 40;
  subnet = "10.10.40.0";
  subnetPrefix = 24;
  staticAddresses = {
    gateway = "10.10.40.1";
    server = "10.10.40.2";
  };
  domain = "guest.internal";

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
  den.aspects.infra._.networks._.guest = {
    aspect-name = "infra.networks.guest";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
    ];

    os = _: {
      infra.vlans.guest = vlanConfig;
    };

    terranix =
      { config, ... }:
      {
        infra.vlans.guest = vlanConfig;

        resource.unifi_network.guest = {
          name = "GUEST";
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
            to = "unifi_network.guest";
            id = "GUEST";
          }
        ];
      };
  };
}
