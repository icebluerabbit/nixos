/*
  _mk-wan.nix — constructor for `infra.wans.*` aspects (an uplink).

  Underscore-prefixed so import-tree skips it: this is a function, not a module.

  Defaults describe a DHCP uplink with IPv6 and IGMP proxying off. `loadBalance`
  matters only once a second WAN exists — until then the weight is inert, but the
  controller stores it either way.
*/
{ den }:
{
  id,
  displayName,
  unifiId,
  uploadKbps,
  downloadKbps,
  type ? "dhcp",
  loadBalanceType ? "weighted",
  loadBalanceWeight ? 99,
  failoverPriority ? 1,
  resourceName,
  extra ? { },
}:
{
  den.aspects.infra._.wans._.${id} = {
    aspect-name = "infra.wans.${id}";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
    ];

    terranix = {
      resource.unifi_wan.${resourceName} = {
        name = displayName;
        inherit type;
        enabled = true;
        load_balance = {
          type = loadBalanceType;
          weight = loadBalanceWeight;
          failover_priority = failoverPriority;
        };
        setting_preference = "manual";
        report_wan_event = false;
        provider_capabilities = {
          upload_kilobits_per_second = uploadKbps;
          download_kilobits_per_second = downloadKbps;
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
      }
      // extra;

      import = [
        {
          to = "unifi_wan.${resourceName}";
          id = unifiId;
        }
      ];
    };
  };
}
