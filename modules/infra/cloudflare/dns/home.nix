{ den, ... }:
{
  den.aspects.infra._.cloudflare._.dns._.home = {
    aspect-name = "infra.cloudflare.dns.home";
    includes = [
      den.aspects.infra._.providers._.cloudflare
    ];

    terranix = {
      resource.cloudflare_dns_record.home = {
        zone_id = "\${resource.cloudflare_zone.primary.id}";
        name = "\${var.cf_record_name}";
        type = "A";
        content = "\${var.wan_ip}";
        proxied = false;
        ttl = 1;
        lifecycle = {
          ignore_changes = [ "content" ];
        };
      };

      import = [
        {
          to = "cloudflare_dns_record.home";
          id = "\${var.cf_zone_id}/467a8bd707ee79cb55712bfc68f8744e";
        }
      ];
    };
  };
}
