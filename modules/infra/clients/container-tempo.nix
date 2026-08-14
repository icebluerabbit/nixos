# modules/infra/clients/container-tempo.nix
{ den, lib, ... }:
(import ../_mk-client.nix { inherit den lib; }) {
  id = "container-tempo";
  resourceName = "container_tempo";
  name = "Tempo Container";
  mac = "62:f5:a4:5b:20:98";
  vlan = "services";
  addressKey = "tempo";
  ports = {
    default = 3200;
    otlp-grpc = 4317;
    otlp-http = 4318;
  };
  proxy.port = "default";
}
