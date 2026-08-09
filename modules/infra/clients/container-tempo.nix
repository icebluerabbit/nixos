# modules/infra/clients/container-tempo.nix
{ den, ... }:
let
  name = "Tempo Container";
  mac = "62:f5:a4:5b:20:98";
in
{
  den.aspects.infra._.clients._.container-tempo = {
    aspect-name = "infra.clients.container-tempo";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
    ];

    os =
      { config, ... }:
      let
        ip = config.infra.vlans.services.staticAddresses.tempo;
      in
      {
        infra.clients.container-tempo = {
          inherit name mac ip;
          ports = {
            default = 3200;
            otlp-grpc = 4317;
            otlp-http = 4318;
          };
          proxy.port = "default";
        };
      };

    terranix =
      { config, ... }:
      let
        ip = config.infra.vlans.services.staticAddresses.tempo;
      in
      {
        infra.clients.container-tempo = {
          inherit name mac ip;
          ports = {
            default = 3200;
            otlp-grpc = 4317;
            otlp-http = 4318;
          };
          proxy.port = "default";
        };

        resource.unifi_client.container_tempo = {
          inherit mac name;
          fixed_ip = ip;
          allow_existing = true;
          blocked = false;
          site = "default";
          skip_forget_on_destroy = false;
        };

        import = [
          {
            to = "unifi_client.container_tempo";
            id = mac;
          }
        ];
      };
  };
}
