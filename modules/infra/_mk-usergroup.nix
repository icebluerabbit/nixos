/*
  _mk-usergroup.nix — constructor for `infra.usergroups.*` aspects.

  Underscore-prefixed so import-tree skips it: this is a function, not a module.

  A user group is the controller's bandwidth-limit bucket. Omitting the rate
  fields leaves it unlimited, which is what the default group is for; pass
  `extra` to cap one.
*/
_:
{
  displayName,
  unifiId,
  resourceName,
  extra ? { },
}:
{
  mkTerranix = {
    resource.unifi_client_qos_rate.${resourceName} = {
      name = displayName;
      site = "default";
    }
    // extra;

    import = [
      {
        to = "unifi_client_qos_rate.${resourceName}";
        id = unifiId;
      }
    ];
  };
}
