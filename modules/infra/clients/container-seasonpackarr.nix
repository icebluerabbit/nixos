# home network
{ den, lib, ... }:
(import ../_mk-client.nix { inherit den lib; }) {
  id = "container-seasonpackarr";
  resourceName = "container_seasonpackarr";
  name = "seasonpackarr Container";
  mac = "62:f5:a4:5b:20:9a";
  vlan = "media";
  addressKey = "seasonpackarr";
  ports.default = 42069;
  # No proxy.port: seasonpackarr serves only /api/pack, /api/parse and
  # /api/healthz — there is no web UI to reverse-proxy.
}
