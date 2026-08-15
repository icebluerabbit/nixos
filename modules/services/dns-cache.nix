/*
  dns-cache — local caching split-forwarder: internal zones go to the homelab
  resolver, everything else to Pi-hole for filtering.

  systemd-resolved's failover is sticky: given DNS=<internal> <public>, one
  timeout moves Current DNS Server to the public one and it never moves back
  without a restart. A public resolver cannot answer *.internal, so internal
  resolution stays broken until systemd-resolved is restarted by hand. Routing
  by domain instead of by server order removes that failure mode entirely, and
  keeps public lookups independent of homelab load.

  Splitting here rather than pointing straight at Pi-hole is what keeps internal
  resolution alive when Pi-hole is down: *.internal never touches it.
*/
{ den, ... }:
{
  den.aspects.services._.dns-cache = {
    aspect-name = "services.dns-cache";
    includes = [
      den.aspects.infra._.clients._.container-dns
      den.aspects.infra._.clients._.container-pi-hole
    ];

    nixos =
      { config, ... }:
      {
        services.dnsmasq = {
          enable = true;
          # systemd-resolved owns /etc/resolv.conf and 127.0.0.53.
          resolveLocalQueries = false;
          settings = {
            # Not 127.0.0.53 or .54 — systemd-resolved binds both (stub and proxy stub).
            listen-address = "127.0.0.55";
            bind-interfaces = true;
            no-resolv = true;
            # Pi-hole is the sole default forwarder on purpose. dnsmasq queries every
            # server in a strata and takes the first reply, so listing a public
            # resolver alongside it would race — Cloudflare wins often enough that
            # blocked domains resolve anyway. Losing Pi-hole costs public lookups but
            # not *.internal, which is the failure mode worth keeping.
            server = [
              "/internal/${config.infra.clients.container-dns.ip}"
              config.infra.clients.container-pi-hole.ip
            ];
            rev-server = "10.0.0.0/8,${config.infra.clients.container-dns.ip}";
            cache-size = 10000;
            min-cache-ttl = 120;
            # No bogus-priv here: it would race rev-server over RFC1918 PTR queries and
            # can answer NXDOMAIN for internal reverse lookups. rev-server already keeps
            # them off the public forwarders, which is the only leak that mattered.
            domain-needed = true;
          };
        };
      };
  };
}
