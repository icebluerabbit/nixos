# Loki datasource. uid pinned to its existing (postgres) uid so provisioning
# updates in place; derivedFields link trace ids → the Tempo datasource by its
# real uid (name-refs only resolve by fallback and break in strict contexts).
{ config }:
{
  name = "Loki";
  uid = "P8E80F9AEF21F6940";
  type = "loki";
  access = "proxy";
  url = "http://${config.infra.clients.container-loki.ip}:${toString config.infra.clients.container-loki.ports.default}";
  isDefault = true;
  jsonData.derivedFields = [
    {
      datasourceUid = "P214B5B846CF3925F"; # Tempo
      matcherRegex = "(?:trace_id|traceID|traceId)=(\\w+)";
      name = "Trace ID";
      url = "$${__value.raw}";
    }
  ];
}
