# modules/infra/networks/apps.nix
{ den, ... }:
let
  vlanId = 110;
  subnet = "10.10.110.0";
  subnetPrefix = 24;
  staticAddresses = {
    gateway = "10.10.110.1";
    server = "10.10.110.2";
    # 10.10.110.3 — RESERVED / decommissioned (ex-flood, removed 2026-07, replaced
    # by qui at .11). No container backs it. Future LLMs: do NOT assign .3 to a new
    # container — skip it even though next_free_ip will offer it (the slot is free).
    caddy = "10.10.110.4";
    homepage = "10.10.110.5";
    kiwix = "10.10.110.6";
    gitea = "10.10.110.7";
    openchamber = "10.10.110.8";
    cinny = "10.10.110.9";
    thelounge = "10.10.110.10";
    qui = "10.10.110.11";
    code-server = "10.10.110.3";
  };
  domain = "apps.internal";

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
  den.aspects.infra._.networks._.apps = {
    aspect-name = "infra.networks.apps";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
    ];

    os = _: {
      infra.vlans.apps = vlanConfig;
    };

    terranix =
      { config, ... }:
      {
        infra.vlans.apps = vlanConfig;

        resource.unifi_network.apps = {
          name = "APPS";
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
            to = "unifi_network.apps";
            id = "APPS";
          }
        ];
      };
  };
}
