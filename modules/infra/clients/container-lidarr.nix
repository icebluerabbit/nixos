# modules/infra/clients/container-lidarr.nix
{ den, ... }:
let
  name = "Lidarr Container";
  mac = "62:f5:a4:5b:20:7a";
in
{
  den.aspects.infra._.clients._.container-lidarr = {
    aspect-name = "infra.clients.container-lidarr";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
    ];

    os =
      { config, ... }:
      let
        ip = config.infra.vlans.media.staticAddresses.lidarr;
      in
      {
        infra.clients.container-lidarr = {
          inherit
            name
            mac
            ip
            ;
          ports = {
            default = 8686;
            exporter = 9707;
          };
          proxy.port = "default";
        };
      };

    terranix =
      { config, ... }:
      let
        ip = config.infra.vlans.media.staticAddresses.lidarr;
      in
      {
        infra.clients.container-lidarr = {
          inherit
            name
            mac
            ip
            ;
          ports = {
            default = 8686;
            exporter = 9707;
          };
          proxy.port = "default";
        };

        resource.unifi_client.container_lidarr = {
          inherit mac name;
          fixed_ip = ip;
          allow_existing = true;
          blocked = false;
          site = "default";
          skip_forget_on_destroy = false;
        };

        import = [
          {
            to = "unifi_client.container_lidarr";
            id = mac;
          }
        ];
      };
  };
}
