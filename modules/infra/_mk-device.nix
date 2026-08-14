/*
  _mk-device.nix — constructor for `infra.devices.*` aspects (UniFi hardware).

  Underscore-prefixed so import-tree skips it: this is a function, not a module.

  Only the adoption/management envelope is shared across a gateway, an AP and a
  switch, so everything hardware-specific — radio tables, port overrides, STP,
  LCM screen settings — is passed through `extra` verbatim.

  `resourceName` is a stable alias, not a transform of `id`: these resources were
  named by hand and renaming one moves its Terraform address, which destroys and
  recreates the device rather than updating it.
*/
{ den, lib }:
{
  id,
  resourceName,
  name,
  mac,
  ip ? null,
  type ? "dhcp",
  extra ? { },
}:
{
  den.aspects.infra._.devices._.${id} = {
    aspect-name = "infra.devices.${id}";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
    ];

    os = _: {
      infra.devices.${id} = {
        inherit name mac type;
      }
      // lib.optionalAttrs (ip != null) { inherit ip; };
    };

    terranix = {
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
      // extra;

      import = [
        {
          to = "unifi_device.${resourceName}";
          id = mac;
        }
      ];
    };
  };
}
