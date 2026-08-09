# home network
{ den, ... }:
let
  name = "Caddy Container";
  mac = "62:f5:a4:5b:20:70";
in
{
  den.aspects.infra._.clients._.container-caddy = {
    aspect-name = "infra.clients.container-caddy";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
    ];

    os =
      { config, ... }:
      let
        ip = config.infra.vlans.apps.staticAddresses.caddy;
      in
      {
        infra.clients.container-caddy = {
          inherit
            name
            mac
            ip
            ;
          ports = {
            http = 80;
            https = 443;
            admin = 2019;
          };
        };
      };

    terranix =
      { config, ... }:
      let
        ip = config.infra.vlans.apps.staticAddresses.caddy;
      in
      {
        infra.clients.container-caddy = {
          inherit
            name
            mac
            ip
            ;
          ports = {
            http = 80;
            https = 443;
            admin = 2019;
          };
        };

        resource.unifi_client.container_caddy = {
          inherit mac name;
          fixed_ip = ip;
          allow_existing = true;
          blocked = false;
          site = "default";
          skip_forget_on_destroy = false;
        };

        import = [
          {
            to = "unifi_client.container_caddy";
            id = mac;
          }
        ];
      };
  };
}
