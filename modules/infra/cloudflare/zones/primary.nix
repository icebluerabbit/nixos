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

      moved = [
        {
          from = "cloudflare_zone.zms_network";
          to = "cloudflare_zone.primary";
        }
      ];
    };
  };
}
