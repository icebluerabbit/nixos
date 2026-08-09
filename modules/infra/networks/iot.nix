# modules/infra/networks/iot.nix
{ den, ... }:
let
  vlanId = 30;
  subnet = "10.10.30.0";
  subnetPrefix = 24;
  staticAddresses = {
    gateway = "10.10.30.1";
    server = "10.10.30.2";
    home-assistant = "10.10.30.3";
    reterminal-e1003 = "10.10.30.4";
    esphome = "10.10.30.5";
    zwave = "10.10.30.6";
    mosquitto = "10.10.30.7";
    zigbee2mqtt = "10.10.30.8";
  };
  domain = "iot.internal";

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
  den.aspects.infra._.networks._.iot = {
    aspect-name = "infra.networks.iot";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
    ];

    os = _: {
      infra.vlans.iot = vlanConfig;
    };

    terranix =
      { config, ... }:
      {
        infra.vlans.iot = vlanConfig;

        resource.unifi_network.iot = {
          name = "IOT";
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
            to = "unifi_network.iot";
            id = "IOT";
          }
        ];
      };
  };
}
