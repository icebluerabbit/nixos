# modules/infra/clients/container-prometheus.nix
{ den, ... }:
let
  name = "Prometheus Container";
  mac = "62:f5:a4:5b:20:96";
in
{
  den.aspects.infra._.clients._.container-prometheus = {
    aspect-name = "infra.clients.container-prometheus";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
    ];

    os =
      { config, ... }:
      {
        infra.clients.container-prometheus = {
          inherit name mac;
          ip = config.infra.vlans.services.staticAddresses.prometheus;
          ports = {
            default = 9090;
          };
          proxy.port = "default";
        };
      };

    terranix =
      { config, ... }:
      {
        infra.clients.container-prometheus = {
          inherit name mac;
          ip = config.infra.vlans.services.staticAddresses.prometheus;
          ports = {
            default = 9090;
          };
          proxy.port = "default";
        };

        resource.unifi_client.container_prometheus = {
          inherit mac name;
          fixed_ip = config.infra.vlans.services.staticAddresses.prometheus;
          allow_existing = true;
          blocked = false;
          site = "default";
        };

        import = [
          {
            to = "unifi_client.container_prometheus";
            id = mac;
          }
        ];
      };
  };
}
