# home network dynamic DNS configuration
{ den, ... }:
let
  dyndns = (import ../_mk-dyndns.nix null) {
    resourceName = "cloudflare";
    service = "cloudflare";
    unifiId = "6a296aebefa3052c88b2f833";
    hostName = "\${var.cf_record_name}";
    login = "\${var.cf_zone_name}";
    passwordVariable = "dyndns_password";
  };
in
{
  den.aspects.infra._.dyndns._.cloudflare = {
    aspect-name = "infra.dyndns.cloudflare";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.providers._.cloudflare
    ];

    terranix = _: dyndns.mkTerranix;
  };
}
