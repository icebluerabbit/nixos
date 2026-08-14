{ den, lib, ... }:
(import ../../_mk-dns-record.nix { inherit den lib; }) {
  id = "home";
  resourceName = "home";
  zone = "primary";
  name = "\${var.cf_record_name}";
  content = "\${var.wan_ip}";
  recordId = "7159c29d52a721b2ae9aaec71d925bc9";
  zoneIdVariable = "cf_zone_id";
  # The gateway's dynamic DNS rewrites this out of band.
  ignoreContentChanges = true;
}
