/*
  _mk-zone.nix — constructor for `infra.cloudflare.zones.*` aspects.

  Underscore-prefixed so import-tree skips it: this is a function, not a module.

  Zone and account identifiers are sensitive Terraform variables rather than
  literals, so the domain name never lands in the repo. The import id is a
  variable for the same reason.
*/
_:
{
  resourceName,
  nameVariable,
  accountVariable,
  importVariable,
}:
{
  mkTerranix = {
    resource.cloudflare_zone.${resourceName} = {
      name = "\${var.${nameVariable}}";
      account = {
        id = "\${var.${accountVariable}}";
      };
    };

    import = [
      {
        to = "cloudflare_zone.${resourceName}";
        id = "\${var.${importVariable}}";
      }
    ];
  };
}
