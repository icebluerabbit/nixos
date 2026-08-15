# modules/infra/clients/container-tempo.nix
{ den, lib, ... }:
let
  client = (import ../_mk-client.nix { inherit lib; }) {
    id = "container-tempo";
    resourceName = "container_tempo";
    name = "Tempo Container";
    mac = "62:f5:a4:5b:20:98";
    vlan = "services";
    kind = "virtual";
    ports = {
      default = 3200;
      otlp-grpc = 4317;
      otlp-http = 4318;
    };
    proxy.port = "default";
  };
in
{
  den.aspects.infra._.clients._.container-tempo = {
    aspect-name = "infra.clients.container-tempo";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
      den.aspects.infra._.networks._.services
    ];

    os = { config, ... }: client.mkOs config.infra.vlans.services.staticAddresses.tempo;
    terranix = { config, ... }: client.mkTerranix config.infra.vlans.services.staticAddresses.tempo;
  };
}
