/*
  _mk-port-profile.nix — builds the `terranix` body for an `infra.ports.*` aspect.

  Underscore-prefixed so import-tree skips it: this is a function, not a module.

  Plain data in, plain data out — no aspect references, no `config`. `network` is
  the Terraform address of the VLAN this port lands untagged on; the aspect
  declares that VLAN in its `includes`.

  Defaults describe a plain access port: one untagged VLAN, PoE auto, nothing
  else customised. Trunks pass their STP and storm-control settings through
  `extra`.

  `excludedNetworks` is stated once and drives both the registry entry and the
  `excluded_networkconf_ids` Terraform addresses. Writing the interpolations by
  hand alongside the plain list let the two drift, which is silent: the registry
  says one thing and the controller enforces another.

  `unifiId` is the controller object id, and its only job is the `import` block
  that adopts an already-applied profile into state. A profile being created for
  the first time has no such id yet — pass null and no import is emitted. Fill it
  in afterwards if you want the import to stay idempotent on a fresh state.
*/
_:
{
  id,
  resourceName,
  displayName,
  network,
  unifiId ? null,
  extra ? { },
  excludedNetworks ? [ ],
}:
let
  # Mirrored into both engines, like `infra.clients`: the terranix side needs it
  # to resolve a device port's `profile` key to this profile's Terraform address.
  entry = {
    infra.ports.${id} = {
      name = displayName;
      inherit network unifiId resourceName;
      trunk = extra ? tagged_vlan_mgmt;
      inherit excludedNetworks;
    };
  };
in
{
  mkOs = entry;

  mkTerranix = entry // {
    resource.unifi_port_profile.${resourceName} = {
      name = displayName;
      forward = "customize";
      op_mode = "switch";
      poe_mode = "auto";
      native_networkconf_id = "\${unifi_network.${network}.id}";
      site = "default";
    }
    // (
      if excludedNetworks == [ ] then
        { }
      else
        {
          excluded_networkconf_ids = map (n: "\${unifi_network.${n}.id}") excludedNetworks;
        }
    )
    // extra;

    import =
      if unifiId == null then
        [ ]
      else
        [
          {
            to = "unifi_port_profile.${resourceName}";
            id = unifiId;
          }
        ];
  };
}
