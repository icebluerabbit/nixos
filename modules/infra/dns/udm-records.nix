# udm-records — static A records on the gateway's resolver, generated from infra.
#
# Clients that resolve through the gateway itself (Teleport peers) never reach
# the dnsmasq container, and the unifi provider cannot express a forward-domain
# rule, so the gateway carries a copy of the internal names instead. Both sets
# come from the same options dnsmasq and Caddy read, so nothing is listed here.
{ den, lib, ... }:
{
  den.aspects.infra._.dns._.udm-records = {
    aspect-name = "infra.dns.udm-records";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
    ];

    terranix =
      { config, ... }:
      let
        inherit (config.infra) clients vlans;

        # Same fold as the dnsmasq container's dnsRecords.
        hostRecords = lib.foldl' (
          acc: vlan:
          acc // lib.mapAttrs' (name: ip: lib.nameValuePair "${name}.${vlan.domain}" ip) vlan.staticAddresses
        ) { } (lib.attrValues vlans);

        # Same filter Caddy builds its virtual hosts from.
        caddyRecords = lib.mapAttrs' (
          name: _:
          lib.nameValuePair "${lib.removePrefix "container-" name}.caddy.${vlans.apps.domain}" clients.container-caddy.ip
        ) (lib.filterAttrs (_: c: c.proxy != null) clients);
      in
      {
        resource.unifi_dns_record = lib.mapAttrs' (
          fqdn: ip:
          lib.nameValuePair (lib.replaceStrings [ "." ] [ "_" ] fqdn) {
            name = fqdn;
            value = ip;
            record_type = "A";
            site = "default";
          }
        ) (hostRecords // caddyRecords);
      };
  };
}
