/*
  _mk-port-profile.nix — constructor for `infra.ports.*` aspects.

  Underscore-prefixed so import-tree skips it: this is a function, not a module.

  Defaults describe the plain access port: one untagged VLAN, PoE auto, nothing
  else customised. Trunks pass their VLAN-exclusion denylist, STP and
  storm-control settings through `extra`.

  `unifiId` is the controller's own object id for the profile, which is what an
  import has to key on — unlike networks, these have no human-readable
  identifier. It is required: import blocks are idempotent once the resource is
  in state, so every profile keeps one and a missing id is an oversight.
*/
{ den }:
{
  id,
  displayName,
  network,
  unifiId,
  resourceName,
  extra ? { },
}:
{
  den.aspects.infra._.ports._.${id} = {
    aspect-name = "infra.ports.${id}";
    includes = [ den.aspects.infra._.providers._.unifi ];

    terranix = {
      resource.unifi_port_profile.${resourceName} = {
        name = displayName;
        forward = "customize";
        op_mode = "switch";
        poe_mode = "auto";
        native_networkconf_id = "\${unifi_network.${network}.id}";
        site = "default";
      }
      // extra;

      import = [
        {
          to = "unifi_port_profile.${resourceName}";
          id = unifiId;
        }
      ];
    };
  };
}
