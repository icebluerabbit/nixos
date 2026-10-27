/*
  _mk-dns-record.nix — constructor for `infra.cloudflare.dns.*` aspects.

  Underscore-prefixed so import-tree skips it: this is a function, not a module.

  `ttl = 1` is Cloudflare's "automatic", not one second.

  A record whose address is maintained by dynamic DNS should set
  `ignoreContentChanges`: the gateway rewrites the value out of band, and without
  it every plan would show drift and try to push the stale address back.
*/
{ lib }:
{
  resourceName,
  zone,
  name,
  content,
  recordId,
  zoneIdVariable,
  type ? "A",
  proxied ? false,
  ttl ? 1,
  ignoreContentChanges ? false,
}:
{
  mkTerranix = {
    resource.cloudflare_dns_record.${resourceName} = {
      zone_id = "\${resource.cloudflare_zone.${zone}.id}";
      inherit
        name
        type
        content
        proxied
        ttl
        ;
    }
    // lib.optionalAttrs ignoreContentChanges {
      lifecycle = {
        ignore_changes = [ "content" ];
      };
    };

    # cloudflare_dns_record imports as <zone id>/<record id>.
    import = [
      {
        to = "cloudflare_dns_record.${resourceName}";
        id = "\${var.${zoneIdVariable}}/${recordId}";
      }
    ];
  };
}
