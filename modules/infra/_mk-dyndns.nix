/*
  _mk-dyndns.nix — constructor for `infra.dyndns.*` aspects.

  Underscore-prefixed so import-tree skips it: this is a function, not a module.

  The gateway itself pushes the WAN address to the DNS provider, so the record
  tracks a dynamic IP without anything on the LAN polling for it. The password
  is declared as its own sensitive Terraform variable rather than read from the
  provider block, since it is a per-record credential.
*/
_:
{
  service,
  unifiId,
  hostName,
  login,
  passwordVariable,
  interface ? "wan",
  resourceName,
}:
{
  mkTerranix = {
    variable.${passwordVariable} = {
      type = "string";
      sensitive = true;
    };

    resource.unifi_dynamic_dns.${resourceName} = {
      inherit service interface;
      host_name = hostName;
      inherit login;
      password = "\${var.${passwordVariable}}";
      site = "default";
    };

    import = [
      {
        to = "unifi_dynamic_dns.${resourceName}";
        id = unifiId;
      }
    ];
  };
}
