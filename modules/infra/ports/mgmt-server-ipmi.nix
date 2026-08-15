# home network
#
# Split out of the shared MGMT profile so each switch port names exactly one peer.
# Pro HD port 2 is the server's own NIC, port 3 is its BMC; both were on `mgmt`,
# which made the profile ambiguous as a link identity and blocked per-port policy
# from ever diverging.
#
# Same VLAN and same settings as mgmt-server — this is a naming split, not a
# policy change. `unifiId` is the object the first apply created — it only feeds
# the `import` block, so a rebuilt state adopts this profile rather than duplicating it.
{ den, ... }:
let
  port = (import ../_mk-port-profile.nix null) {
    id = "mgmt-server-ipmi";
    resourceName = "mgmt_ipmi";
    displayName = "MGMT Port (IPMI)";
    network = "mgmt";
    unifiId = "6a7f661cabfc0160bad81464";
  };
in
{
  den.aspects.infra._.ports._.mgmt-server-ipmi = {
    aspect-name = "infra.ports.mgmt-server-ipmi";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
      den.aspects.infra._.networks._.mgmt
    ];

    os = _: port.mkOs;
    terranix = _: port.mkTerranix;
  };
}
