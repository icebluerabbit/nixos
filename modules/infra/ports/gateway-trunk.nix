# home network
#
# The uplink from USW Pro HD 24 PoE port 27 to the Dream Machine Pro Max.
#
# This is the switch's own path to the gateway and to the controller. It carried
# no profile at all before this — the port defaulted to forwarding everything —
# and that behaviour is what the settings below reproduce deliberately:
#
#   forward = "all"            every VLAN passes, no denylist
#   tagged_vlan_mgmt = "auto"  controller manages the tag set itself
#
# Do NOT convert this to the `excludedNetworks` denylist the other two trunks
# use. An exclusion that is wrong here removes the switch from the controller,
# and this is the one port where that cannot then be fixed remotely.
#
# native stays Default (untagged) — the switch's own management address
# (10.100.103.119) rides it.
{ den, ... }:
let
  port = (import ../_mk-port-profile.nix null) {
    id = "gateway-trunk";
    resourceName = "gateway_trunk";
    displayName = "Gateway Trunk";
    network = "default";
    unifiId = "6a7f8455abfc0160bad81e6d";
    extra = {
      forward = "all";
      tagged_vlan_mgmt = "auto";
      stp_port_mode = true;
    };
  };
in
{
  den.aspects.infra._.ports._.gateway-trunk = {
    aspect-name = "infra.ports.gateway-trunk";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
      den.aspects.infra._.networks._.default
    ];

    os = _: port.mkOs;
    terranix = _: port.mkTerranix;
  };
}
