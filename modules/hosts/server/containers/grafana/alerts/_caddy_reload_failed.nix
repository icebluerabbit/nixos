# Caddy rejected its config on reload. It keeps serving the last good config, so nothing
# breaks — which is exactly the problem: a deploy that silently didn't take, and every
# route you just changed is still the old one.
{ promRule, ... }:
{
  group = "infra";
  rules = [
    (promRule {
      uid = "caddy_reload_failed";
      title = "Caddy config reload failed";
      expr = "caddy_config_last_reload_successful";
      op = "lt";
      value = 1;
      for = "5m";
      severity = "warning";
      category = "proxy";
      noDataState = "OK";
      summary = "Caddy is running a stale config";
      description = "Caddy's last config reload failed. It is still serving the previous config — any routing change from the last deploy has not taken effect.";
    })
  ];
}
