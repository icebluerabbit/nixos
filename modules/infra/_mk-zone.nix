/*
  _mk-zone.nix — constructor for `infra.cloudflare.zones.*` aspects.

  Underscore-prefixed so import-tree skips it: this is a function, not a module.

  Zone and account identifiers are sensitive Terraform variables rather than
  literals, so the domain name never lands in the repo. The import id is a
  variable for the same reason.
*/
{ den }:
{
  id,
  resourceName,
  nameVariable,
  accountVariable,
  importVariable,
}:
{
  den.aspects.infra._.cloudflare._.zones._.${id} = {
    aspect-name = "infra.cloudflare.zones.${id}";
    includes = [
      den.aspects.infra._.providers._.cloudflare
    ];

    terranix = {
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
  };
}
