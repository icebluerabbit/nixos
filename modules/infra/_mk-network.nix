/*
  _mk-network.nix — builds the `os` and `terranix` bodies for an
  `infra.networks.*` aspect.

  Underscore-prefixed so import-tree skips it: this is a function, not a module.

  Plain data in, plain data out. It never reads `config` and never touches an
  aspect. `mkTerranix` takes the DHCP name servers as an argument, because those
  are usually a client's address and resolving it belongs to the aspect:

      terranix = { config, ... }:
        network.mkTerranix [ config.infra.clients.container-dns.ip ];

  Two shapes come through here:

    - a tagged VLAN: pass `vlanId` and `staticAddresses`. The VLAN is published
      to `infra.vlans.<id>`, the subnet derives from the gateway address, and the
      DHCP pool defaults to `.100`-`.200` of the VLAN's own third octet.
    - the untagged Default network: pass `cidr` instead. No VLAN tag, no static
      allocations, nothing published to `infra.vlans`.
*/
{ lib }:
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

  vlanConfig = dnsServers: {
    name = displayName;
    inherit
      isolated
      dnsServers
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
  mkOs = dnsServers: lib.optionalAttrs isVlan { infra.vlans.${id} = vlanConfig dnsServers; };

  mkTerranix =
    dnsServers:
    lib.optionalAttrs isVlan { infra.vlans.${id} = vlanConfig dnsServers; }
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
          dns_servers = dnsServers;
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
