{ den, ... }:
let
  name = "Cinny Container";
  mac = "62:f5:a4:5b:20:74";
in
{
  den.aspects.infra._.clients._.container-cinny = {
    aspect-name = "infra.clients.container-cinny";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
    ];

    os =
      { config, ... }:
      {
        infra.clients.container-cinny = {
          inherit name mac;
          ip = config.infra.vlans.apps.staticAddresses.cinny;
          ports = {
            default = 80;
          };
          proxy.port = "default";
        };
      };

    terranix =
      { config, ... }:
      {
        infra.clients.container-cinny = {
          inherit name mac;
          ip = config.infra.vlans.apps.staticAddresses.cinny;
          ports = {
            default = 80;
          };
          proxy.port = "default";
        };
        resource.unifi_client.container_cinny = {
          inherit mac name;
          fixed_ip = config.infra.vlans.apps.staticAddresses.cinny;
          allow_existing = true;
          blocked = false;
          site = "default";
          skip_forget_on_destroy = false;
        };
        import = [
          {
            to = "unifi_client.container_cinny";
            id = mac;
          }
        ];
      };
  };
}
