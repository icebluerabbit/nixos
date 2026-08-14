# home network dynamic DNS configuration
{ den, ... }:
(import ../_mk-dyndns.nix { inherit den; }) {
  id = "cloudflare";
  resourceName = "cloudflare";
  service = "cloudflare";
  unifiId = "6a296aebefa3052c88b2f833";
  hostName = "\${var.cf_record_name}";
  login = "\${var.cf_zone_name}";
  passwordVariable = "dyndns_password";
}
