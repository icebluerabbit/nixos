# modules/infra/networks/services.nix
{ den, ... }:
let
  vlanId = 140;
  subnet = "10.10.140.0";
  subnetPrefix = 24;
  staticAddresses = {
    gateway = "10.10.140.1";
    server = "10.10.140.2";
    postgres = "10.10.140.3";
    pgadmin = "10.10.140.4";
    dns = "10.10.140.5";
    pi-hole = "10.10.140.6";
    opencode = "10.10.140.7";
    loki = "10.10.140.8";
    grafana = "10.10.140.9";
    prometheus = "10.10.140.10";
    unifi-exporter = "10.10.140.11";
    tempo = "10.10.140.12";
    authelia = "10.10.140.13";
    tuwunel = "10.10.140.14";
    lldap = "10.10.140.15";
    hermes = "10.10.140.16";
  };
  domain = "services.internal";

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
  den.aspects.infra._.networks._.services = {
    aspect-name = "infra.networks.services";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
    ];

    os = _: {
      infra.vlans.services = vlanConfig;
    };

    terranix =
      { config, ... }:
      {
        infra.vlans.services = vlanConfig;

        resource.unifi_network.services = {
          name = "SERVICES";
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
            to = "unifi_network.services";
            id = "SERVICES";
          }
        ];
      };
  };
}
