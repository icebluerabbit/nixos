# modules/infra/clients/container-grafana.nix
{ den, ... }:
let
  name = "Grafana Container";
  mac = "62:f5:a4:5b:20:95";
in
{
  den.aspects.infra._.clients._.container-grafana = {
    aspect-name = "infra.clients.container-grafana";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
    ];

    os =
      { config, ... }:
      let
        ip = config.infra.vlans.services.staticAddresses.grafana;
      in
      {
        infra.clients.container-grafana = {
          inherit name mac ip;
          ports = {
            default = 3000;
          };
          proxy.port = "default";
        };
      };

    terranix =
      { config, ... }:
      let
        ip = config.infra.vlans.services.staticAddresses.grafana;
      in
      {
        infra.clients.container-grafana = {
          inherit name mac ip;
          ports = {
            default = 3000;
          };
          proxy.port = "default";
        };

        resource.unifi_client.container_grafana = {
          inherit mac name;
          fixed_ip = ip;
          allow_existing = true;
          blocked = false;
          site = "default";
          skip_forget_on_destroy = false;
        };

        import = [
          {
            to = "unifi_client.container_grafana";
            id = mac;
          }
        ];
      };
  };
}
