# home network
{ den, lib, ... }:
let
  network = (import ../_mk-network.nix { inherit lib; }) {
    id = "default";
    resourceName = "default";
    displayName = "Default";
    domain = "localdomain";
    cidr = "10.100.10.1/16";
    dhcpStart = "10.100.0.46";
    dhcpStop = "10.100.255.254";
    conflictChecking = false;
    extra = {
      dhcp_guarding = {
        enabled = false;
      };
      dhcp_relay = {
        enabled = false;
      };
      ipv6_client_address_assignment = "slaac";
      ipv6_pd_auto_prefixid_enabled = false;
      ipv6_pd_start = "::2";
      ipv6_pd_stop = "::7d1";
      ipv6_ra = true;
      ipv6_ra_preferred_lifetime = 14400;
      ipv6_ra_priority = "high";
      lte_lan = false;
    };
  };
in
{
  den.aspects.infra._.networks._.default = {
    aspect-name = "infra.networks.default";
    includes = [
      den.aspects.infra._.providers._.unifi
    ];

    os = _: network.mkOs [ "10.100.10.1" ];
    terranix = _: network.mkTerranix [ "10.100.10.1" ];
  };
}
