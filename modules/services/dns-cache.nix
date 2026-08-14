/*
  dns-cache — local caching split-forwarder: internal zones go to the homelab
  resolver, everything else straight to public DNS.

  systemd-resolved's failover is sticky: given DNS=<internal> <public>, one
  timeout moves Current DNS Server to the public one and it never moves back
  without a restart. A public resolver cannot answer *.internal, so internal
  resolution stays broken until systemd-resolved is restarted by hand. Routing
  by domain instead of by server order removes that failure mode entirely, and
  keeps public lookups independent of homelab load.
*/
{ den, ... }:
{
  den.aspects.services._.dns-cache = {
    aspect-name = "services.dns-cache";
    includes = [ den.aspects.infra._.clients._.container-dns ];

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
            server = [
              "/internal/${config.infra.clients.container-dns.ip}"
              "1.1.1.1"
              "1.0.0.1"
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
