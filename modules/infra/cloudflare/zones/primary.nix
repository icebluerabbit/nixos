{ den, ... }:
let
  zone = (import ../../_mk-zone.nix null) {
    resourceName = "primary";
    nameVariable = "cf_zone_name";
    accountVariable = "cf_account_id";
    importVariable = "cf_zone_id";
  };
in
{
  den.aspects.infra._.cloudflare._.zones._.primary = {
    aspect-name = "infra.cloudflare.zones.primary";
    includes = [
      den.aspects.infra._.providers._.cloudflare
    ];

    terranix = _: zone.mkTerranix;
  };
}
