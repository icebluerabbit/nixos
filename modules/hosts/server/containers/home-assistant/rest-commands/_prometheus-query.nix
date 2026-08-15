# Instant PromQL query against Prometheus — used by the digest automations to read
# numbers at send time rather than carrying them through the event payload.
#
# POST, not GET: rest_command sends `payload` as the request body, and Prometheus's
# GET handler only reads URL parameters — so a GET here arrives with no query at all
# ("no expression found in input") no matter what the payload renders to. The POST
# form-encoded endpoint is also the documented route for selectors long enough to
# strain a URL, which the digest's __name__ regex is.
#
# Prometheus here is fed by remote_write and has no `up` series, so a query that
# matches nothing returns an empty `result` array rather than an error; callers must
# tolerate that (`| default` in the templates) instead of assuming a value.
{ config, ... }:
{
  prometheus_query = {
    url = "http://${config.infra.clients.container-prometheus.ip}:${toString config.infra.clients.container-prometheus.ports.default}/api/v1/query";
    method = "POST";
    payload = "query={{ query | urlencode }}";
    content_type = "application/x-www-form-urlencoded";
  };
}
