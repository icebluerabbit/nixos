# home network dynamic DNS configuration
{ den, ... }:
{
  den.aspects.infra._.dyndns._.cloudflare = {
    aspect-name = "infra.dyndns.cloudflare";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.providers._.cloudflare
    ];
    terranix = {
      variable.dyndns_password = {
        type = "string";
        sensitive = true;
      };

      resource.unifi_dynamic_dns.cloudflare = {
        service = "cloudflare";
        host_name = "\${var.cf_record_name}";
        login = "\${var.cf_zone_name}";
        password = "\${var.dyndns_password}";
        interface = "wan";
        site = "default";
      };

      import = [
        {
          to = "unifi_dynamic_dns.cloudflare";
          id = "6a296aebefa3052c88b2f833";
        }
      ];
    };
  };
}
