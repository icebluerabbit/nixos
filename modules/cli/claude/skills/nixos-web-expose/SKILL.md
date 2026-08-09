---
name: nixos-web-expose
description: Expose a container's web UI on this homelab — reverse-proxy it through Caddy and add a Homepage dashboard card. Use whenever the user asks to make a service reachable by name, add a reverse proxy entry, put something behind Caddy, or add a service to the homepage/dashboard. DNS is automatic. Assumes the container already exists (see nixos-container-create). Also load nixos-secret-add when a Homepage widget needs an API key, and nixos-infra-provision for external Cloudflare DNS / port-forwards.
---

# Expose a Web Service (Caddy + Homepage)

The wiring is largely **automated**. Do not hand-write DNS records or Caddy virtualHosts — they are generated from the infra registry.

## 1. DNS — automatic, nothing to do

`modules/hosts/server/containers/dnsmasq.nix` folds **every** VLAN's `staticAddresses` into records: `"${name}.${vlan.domain}" = ip`. Allocating a static IP in `modules/infra/networks/<vlan>.nix` (which `nixos-container-create` already does) automatically creates `<name>.<vlan-domain>`. All VLAN aspects are already in dnsmasq's `includes`. Only act here if adding a brand-new VLAN (add its network aspect to dnsmasq `includes`).

## 2. Caddy — set `proxy` on the infra client

Caddy auto-generates a virtualHost for **every** infra client that has a `proxy` set:
```nix
# caddy.nix (do NOT edit): virtualHosts = filterAttrs (c: c.proxy != null) infra.clients
#   → subdomain = client name minus "container-",  port = client.ports.${client.proxy.port}
```

So to proxy a service, edit its infra client `modules/infra/clients/container-<name>.nix` and add `proxy.port` (the named port **key** from `ports`) — in **both** the `nixos` and `terranix` sections:
```nix
infra.clients.container-radarr = {
  inherit name mac ip;
  ports = { default = 7878; };
  proxy.port = "default";        # ← key into ports; exposes radarr.caddy.<apps-domain>
};
```
Multi-port: point at the right key, e.g. `proxy.port = "http";` or `"web"` / `"ui"`.

Then add the service to Caddy's `includes` in `modules/hosts/server/containers/caddy.nix` so its client enters `config.infra.clients` scope — follow the existing pattern, **both** lines:
```nix
den.aspects.containers._.radarr
den.aspects.infra._.clients._.container-radarr
```
Result: `http://radarr.caddy.<apps-domain>/` → `http://<ip>:<port>`. (The client/proxy is what drives the vhost; the container-aspect line matches the established convention.)

## 3. Homepage — still manual

`modules/hosts/server/containers/homepage.nix` is a hand-curated `services` list. To add a card:
1. Add the service's container + infra client to homepage's `includes` (both lines, as in caddy).
2. Add an entry under the right group (`Apps`/`Network`/`Services`/`Media`) in `containerConfigs.homepage.services`:
   ```nix
   {
     "Radarr" = {
       icon = "radarr.svg";
       href = "http://radarr.caddy.${config.infra.vlans.apps.domain}/";
       # optional widget (needs an API key via the homepage-env sops template — see nixos-secret-add):
       widget = {
         type = "radarr";
         url = "http://${config.infra.clients.container-radarr.ip}:${toString config.infra.clients.container-radarr.ports.default}";
         key = "{{HOMEPAGE_VAR_RADARR_KEY}}";
         fields = [ "wanted" "missing" "queued" "movies" ];
       };
     };
   }
   ```
   Widget API keys are injected via `HOMEPAGE_VAR_*` from the `homepage-env` sops template — add the secret with `nixos-secret-add` if using a widget. Plain `href`+`description` cards need no secret.

## Finish (zero-effect commands only)
```bash
git add -N <changed files>
nix fmt
just eval-server        # validate
just build-server       # optional dry-run build
```
**Deploy is human-only.** The assistant never runs `switch`/`test`/`apply` — hand the user `just switch-server`.

## Verify after the user deploys
```bash
curl -s http://<container-ip>:<port>/          # bridged IP, routable from any LAN host
# or, once live:  http://<name>.caddy.<apps-domain>/
```

## Gotchas
- Name doesn't resolve → the static IP/VLAN isn't registered, or a new VLAN missing from dnsmasq `includes`.
- Resolves but 502 → `proxy.port` points at a wrong/undeclared port key, or the service isn't in Caddy's `includes`.
- `proxy.port` is a **string key** into `ports`, not a number.
