# modules/infra/clients/container-radarr.nix
{ den, ... }:
let
  name = "Radarr Container";
  mac = "62:f5:a4:5b:20:79";
in
{
  den.aspects.infra._.clients._.container-radarr = {
    aspect-name = "infra.clients.container-radarr";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options

    ];

    os =
      { config, ... }:
      let
        ip = config.infra.vlans.media.staticAddresses.radarr;
      in
      {
        infra.clients.container-radarr = {
          inherit
            name
            mac
            ip
            ;
          ports = {
            default = 7878;
            exporter = 9707;
          };
          proxy.port = "default";
        };
      };

    terranix =
      { config, ... }:
      let
        ip = config.infra.vlans.media.staticAddresses.radarr;
      in
      {
        infra.clients.container-radarr = {
          inherit
            name
            mac
            ip
            ;
          ports = {
            default = 7878;
            exporter = 9707;
          };
          proxy.port = "default";
        };

        resource.unifi_client.container_radarr = {
          inherit mac name;
          fixed_ip = ip;
          allow_existing = true;
          blocked = false;
          site = "default";
          skip_forget_on_destroy = false;
        };

        import = [
          {
            to = "unifi_client.container_radarr";
            id = mac;
          }
        ];
      };
  };
}
