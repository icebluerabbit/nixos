# qui — modern qBittorrent WebUI (replaces flood)
{ den, ... }:
let
  name = "qui Container";
  mac = "62:f5:a4:5b:20:77";
in
{
  den.aspects.infra._.clients._.container-qui = {
    aspect-name = "infra.clients.container-qui";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
    ];

    os =
      { config, ... }:
      let
        ip = config.infra.vlans.apps.staticAddresses.qui;
      in
      {
        infra.clients.container-qui = {
          inherit
            name
            mac
            ip
            ;
          ports = {
            default = 7476;
          };
          proxy.port = "default";
        };
      };

    terranix =
      { config, ... }:
      let
        ip = config.infra.vlans.apps.staticAddresses.qui;
      in
      {
        infra.clients.container-qui = {
          inherit
            name
            mac
            ip
            ;
          ports = {
            default = 7476;
          };
          proxy.port = "default";
        };

        resource.unifi_client.container_qui = {
          inherit mac name;
          fixed_ip = ip;
          allow_existing = true;
          blocked = false;
          site = "default";
          skip_forget_on_destroy = false;
        };

        import = [
          {
            to = "unifi_client.container_qui";
            id = mac;
          }
        ];
      };
  };
}
