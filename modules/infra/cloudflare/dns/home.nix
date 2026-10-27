{ den, lib, ... }:
let
  record = (import ../../_mk-dns-record.nix { inherit lib; }) {
    resourceName = "home";
    zone = "primary";
    name = "\${var.cf_record_name}";
    content = "\${var.wan_ip}";
    recordId = "7159c29d52a721b2ae9aaec71d925bc9";
    zoneIdVariable = "cf_zone_id";
    # The gateway's dynamic DNS rewrites this out of band.
    ignoreContentChanges = true;
  };
in
{
  den.aspects.infra._.cloudflare._.dns._.home = {
    aspect-name = "infra.cloudflare.dns.home";
    includes = [
      den.aspects.infra._.providers._.cloudflare
      den.aspects.infra._.cloudflare._.zones._.primary
    ];

    terranix = _: record.mkTerranix;
  };
}
