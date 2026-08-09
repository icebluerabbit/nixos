# Force a vpn-gw broker client onto a fresh tunnel WITHOUT marking the current one unhealthy.
# The broker frees the client's claim and records a short-lived exclusion, so the client's next
# keepalive tick (≤ its cadence) re-claims a different tunnel. Pure selection steering — no health
# or flap signal. Keyed by client_name (the broker-side label), defaulting to qbittorrent; pass
# {"client_name": "..."} as call data to rotate a different broker client (slskd, etc.).
# No authz on the broker control port (trusted VLAN) — reachability is a UniFi firewall concern
# (HA's VLAN → vpn-gw:control), not an app-level one.
{ config, ... }:
{
  vpn_rotate = {
    url = "http://${config.infra.clients.container-vpn-gw.ip}:${toString config.infra.clients.container-vpn-gw.ports.control}/rotate";
    method = "POST";
    content_type = "application/json";
    payload = "{\"client_name\": \"{{ client_name | default('qbittorrent') }}\"}";
  };
}
