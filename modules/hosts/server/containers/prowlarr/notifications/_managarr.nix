# managarr recorder — a SECOND Connect entry alongside _home-assistant.nix.
#
# Prowlarr's event surface is thin: a grab it proxied, plus health and version
# events. There may be nothing per-release worth keeping here at all — wiring
# it is how we find that out rather than assuming.
{ config, ... }:
let
  m = config.infra.clients.container-managarr;
in
{
  name = "managarr";
  implementation = "Webhook";
  url = "http://${m.ip}:${toString m.ports.default}/hook/prowlarr";
  method = 1; # POST
  on_grab = true;
  on_health_issue = true;
  on_health_restored = true;
  on_application_update = true;
  include_health_warnings = true;
}
