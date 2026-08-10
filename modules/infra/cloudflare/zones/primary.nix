{ den, ... }:
{
  den.aspects.infra._.cloudflare._.zones._.primary = {
    aspect-name = "infra.cloudflare.zones.primary";
    includes = [
      den.aspects.infra._.providers._.cloudflare
    ];

    terranix = {
      resource.cloudflare_zone.primary = {
        name = "\${var.cf_zone_name}";
        account = {
          id = "\${var.cf_account_id}";
        };
      };

      import = [
        {
          to = "cloudflare_zone.primary";
          id = "\${var.cf_zone_id}";
        }
      ];
    };
  };
}
