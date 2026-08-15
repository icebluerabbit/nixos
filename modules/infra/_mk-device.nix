/*
  _mk-device.nix — constructor for `infra.devices.*` aspects (UniFi hardware).

  Underscore-prefixed so import-tree skips it: this is a function, not a module.

  Only the adoption/management envelope is shared across a gateway, an AP and a
  switch, so the rest of the hardware-specific settings — radio tables, STP, LCM
  screen — pass through `extra` verbatim.

  `resourceName` is a stable alias, not a transform of `id`: these resources were
  named by hand and renaming one moves its Terraform address, which destroys and
  recreates the device rather than updating it.

  `portProfile` names the profile on the *upstream* switch port this device plugs
  into. It produces no Terraform, only the topology edge; the switch at the other
  end owns the matching port.

  `ports` are this device's own switch ports, keyed by index:

      ports."2" = { name = "Server"; profile = "mgmt-server"; };

  `profile` is an `infra.ports` key — the same name the aspect includes and the
  same name a client puts in its `portProfile`. The Terraform address it maps to
  is not known here: `mkTerranix` takes the resolved `infra.ports` registry and
  looks it up, the way an aspect hands a client its resolved address. So the key
  is what the registry stores and what topology joins on, and the
  `${unifi_port_profile.<x>.id}` interpolation exists only in generated output.

  Ports the switch owns but does not configure need no entry: a LAG member is
  claimed by `aggregateMembers` on the lead port.
*/
{ lib }:
{
  id,
  resourceName,
  name,
  mac,
  ip ? null,
  type ? "dhcp",
  portProfile ? null,
  ports ? { },
  extra ? { },
}:
let
  byIndex = lib.sort (a: b: lib.toInt a < lib.toInt b) (lib.attrNames ports);

  # Terraform wants a list and treats it as a set, but emitting it in index order
  # keeps the generated JSON stable and diffable.
  overridesFor =
    profiles:
    map (
      index:
      let
        p = ports.${index};
      in
      {
        index = lib.toInt index;
      }
      // lib.optionalAttrs (p.name or null != null) { inherit (p) name; }
      // lib.optionalAttrs (p.profile or null != null) {
        port_profile_id = "\${unifi_port_profile.${profiles.${p.profile}.resourceName}.id}";
      }
      // lib.optionalAttrs (p.aggregateMembers or [ ] != [ ]) {
        op_mode = "aggregate";
        aggregate_members = p.aggregateMembers;
      }
      // (p.extra or { })
    ) byIndex;
in
{
  mkOs = {
    infra.devices.${id} = {
      inherit
        name
        mac
        type
        portProfile
        ;
      ports = lib.mapAttrs (_: p: {
        name = p.name or "";
        profile = p.profile or null;
        aggregateMembers = p.aggregateMembers or [ ];
      }) ports;
    }
    // lib.optionalAttrs (ip != null) { inherit ip; };
  };

  # `profiles` is `config.infra.ports`, passed in by the aspect. The helper reads
  # no config and knows no aspects; it only needs the key → Terraform address map
  # that the registry already holds.
  mkTerranix = profiles: {
    resource.unifi_device.${resourceName} = {
      inherit mac name;
      allow_adoption = true;
      forget_on_destroy = true;
      site = "default";
      disabled = false;
      flowctrl_enabled = false;
      jumboframe_enabled = false;
      lcm_brightness_override = false;
      lcm_idle_timeout_override = false;
      outlet_enabled = false;
      config_network = {
        bonding_enabled = false;
        inherit type;
      }
      // lib.optionalAttrs (ip != null) { inherit ip; };
    }
    // lib.optionalAttrs (ports != { }) { port_override = overridesFor profiles; }
    // extra;

    import = [
      {
        to = "unifi_device.${resourceName}";
        id = mac;
      }
    ];
  };
}
