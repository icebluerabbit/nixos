# Caddy upstream health. One rule covering every reverse-proxied web UI: Caddy already
# health-checks each upstream, so this is the cheapest possible "is the service its
# users actually hit still answering" signal, and the `upstream` label names the
# ip:port that failed.
#
# noDataState=Alerting: silence here means Caddy itself stopped publishing, which
# takes down every proxied service at once — strictly worse than one bad upstream.
{ promRule, ... }:
{
  group = "infra";
  rules = [
    (promRule {
      uid = "web_service_down";
      title = "Web service unreachable";
      expr = "caddy_reverse_proxy_upstreams_healthy";
      op = "lt";
      value = 1;
      for = "5m";
      severity = "critical";
      category = "proxy";
      noDataState = "Alerting";
      # A dead upstream's series lingers; give it a few evals before auto-resolving so a
      # restart loop doesn't read as resolved between crashes.
      missingSeriesEvalsToResolve = 3;
      summary = "Caddy upstream {{ $labels.upstream }} unhealthy";
      description = "Caddy's health check for upstream {{ $labels.upstream }} has been failing for 5m — the service behind it is down or refusing connections.";
    })
  ];
}
