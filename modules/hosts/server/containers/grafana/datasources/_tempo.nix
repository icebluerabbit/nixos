# Tempo datasource. uid pinned to its existing (postgres) uid; traces↔logs links
# reference the Loki datasource by its real uid.
{ config }:
{
  name = "Tempo";
  uid = "P214B5B846CF3925F";
  type = "tempo";
  access = "proxy";
  url = "http://${config.infra.clients.container-tempo.ip}:${toString config.infra.clients.container-tempo.ports.default}";
  jsonData = {
    lokiSearch.datasourceUid = "P8E80F9AEF21F6940"; # Loki
    tracesToLogs = {
      datasourceUid = "P8E80F9AEF21F6940"; # Loki
      spanStartTimeShift = "1h";
      spanEndTimeShift = "1h";
      filterBySpanID = false;
      filterByTraceID = true;
      tags = [ "host" ];
      mapTagNamesEnabled = true;
    };
  };
}
