/*
  _mk-network.nix — constructor for `infra.networks.*` aspects.

  Underscore-prefixed so import-tree skips it: this is a function, not a module.

  Two shapes come through here:

    - A tagged VLAN: pass `vlanId` and `staticAddresses`. The VLAN is published
      to `infra.vlans.<id>` for both engines, the subnet is derived from the
      gateway address, and the DHCP pool is `.100`-`.200` of the VLAN's own third
      octet — which holds because every VLAN here is `10.10.<vlanId>.0/24`.

    - The untagged Default network: pass `cidr` instead. It carries no VLAN tag
      and no static allocations, so nothing is published to `infra.vlans` and the
      aspect does not pull in the options module.

  Anything a given network needs beyond the common envelope — IPv6 prefix
  delegation, DHCP guarding — rides in `extra`.
*/
{ den, lib }:
{
  id,
  resourceName,
  displayName,
  domain,
  vlanId ? null,
  staticAddresses ? null,
  subnet ? null,
  subnetPrefix ? 24,
  cidr ? null,
  isolated ? false,
  dnsServers ? null,
  dhcpStart ? "10.10.${toString vlanId}.100",
  dhcpStop ? "10.10.${toString vlanId}.200",
  leasetime ? 86400,
  conflictChecking ? true,
  extra ? { },
}:
assert lib.assertMsg (
  (staticAddresses != null) != (cidr != null)
) "mkNetwork ${id}: pass staticAddresses (a VLAN) or cidr (the untagged default), not both";
let
  isVlan = staticAddresses != null;

  vlanConfig = {
    inherit
      vlanId
      subnet
      subnetPrefix
      domain
      staticAddresses
      dhcpStart
      dhcpStop
      ;
  };
in
{
  den.aspects.infra._.networks._.${id} = {
    aspect-name = "infra.networks.${id}";
    includes = [
      den.aspects.infra._.providers._.unifi
    ]
    ++ lib.optional isVlan den.aspects.infra._.options;

    terranix =
      { config, ... }:
      lib.optionalAttrs isVlan {
        infra.vlans.${id} = vlanConfig;
      }
      // {
        resource.unifi_network.${resourceName} = {
          name = displayName;
          subnet = if isVlan then "${staticAddresses.gateway}/${toString subnetPrefix}" else cidr;
          domain_name = domain;
          auto_scale = false;
          enabled = true;
          gateway_type = "default";
          igmp_snooping = false;
          internet_access = true;
          ipv6_interface_type = "none";
          multicast_dns = true;
          network_isolation = isolated;
          setting_preference = "manual";
          third_party_gateway = false;
          site = "default";
          dhcp_server = {
            boot = {
              enabled = false;
            };
            conflict_checking = conflictChecking;
            dns_enabled = true;
            dns_servers = if dnsServers != null then dnsServers else [ config.infra.clients.container-dns.ip ];
            enabled = true;
            gateway_enabled = false;
            inherit leasetime;
            ntp_enabled = false;
            start = dhcpStart;
            stop = dhcpStop;
            time_offset_enabled = false;
            wins = {
              enabled = false;
            };
          };
        }
        // lib.optionalAttrs isVlan { vlan = vlanId; }
        // extra;

        import = [
          {
            to = "unifi_network.${resourceName}";
            id = displayName;
          }
        ];
      };
  }
  // lib.optionalAttrs isVlan {
    os = _: {
      infra.vlans.${id} = vlanConfig;
    };
  };
}
