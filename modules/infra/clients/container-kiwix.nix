# modules/infra/clients/container-kiwix.nix
{ den, ... }:
let
  name = "Kiwix Container";
  mac = "62:f5:a4:5b:20:72";
in
{
  den.aspects.infra._.clients._.container-kiwix = {
    aspect-name = "infra.clients.container-kiwix";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
    ];

    os =
      { config, ... }:
      let
        ip = config.infra.vlans.apps.staticAddresses.kiwix;
      in
      {
        infra.clients.container-kiwix = {
          inherit
            name
            mac
            ip
            ;
          ports = {
            http = 8080;
          };
          proxy.port = "http";
        };
      };

    terranix =
      { config, ... }:
      let
        ip = config.infra.vlans.apps.staticAddresses.kiwix;
      in
      {
        infra.clients.container-kiwix = {
          inherit
            name
            mac
            ip
            ;
          ports = {
            http = 8080;
          };
          proxy.port = "http";
        };

        resource.unifi_client.container_kiwix = {
          inherit mac name;
          fixed_ip = ip;
          allow_existing = true;
          blocked = false;
          site = "default";
          skip_forget_on_destroy = false;
        };

        import = [
          {
            to = "unifi_client.container_kiwix";
            id = mac;
          }
        ];
      };
  };
}
