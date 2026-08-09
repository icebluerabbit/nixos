# modules/infra/networks/media.nix
{ den, ... }:
let
  vlanId = 120;
  subnet = "10.10.120.0";
  subnetPrefix = 24;
  staticAddresses = {
    gateway = "10.10.120.1";
    server = "10.10.120.2";
    radarr = "10.10.120.4";
    lidarr = "10.10.120.5";
    readarr = "10.10.120.6";
    lazylibrarian = "10.10.120.3";
    calibre-web = "10.10.120.17";
    sonarr = "10.10.120.7";
    prowlarr = "10.10.120.8";
    youtarr = "10.10.120.9";
    jellyfin = "10.10.120.10";
    unpackerr = "10.10.120.11";
    soularr = "10.10.120.19";
    cross-seed = "10.10.120.15";
    bazarr = "10.10.120.16";
    flaresolverr = "10.10.120.13";
    autobrr = "10.10.120.14";
    beets = "10.10.120.18";
    # ⚠ Do NOT assign 10.10.120.12 to any container — reserved.
    mousetrap = "10.10.120.20";
    cleanuparr = "10.10.120.21";
  };
  domain = "media.internal";

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
  den.aspects.infra._.networks._.media = {
    aspect-name = "infra.networks.media";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
    ];

    os = _: {
      infra.vlans.media = vlanConfig;
    };

    terranix =
      { config, ... }:
      {
        infra.vlans.media = vlanConfig;

        resource.unifi_network.media = {
          name = "MEDIA";
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
            to = "unifi_network.media";
            id = "MEDIA";
          }
        ];
      };
  };
}
