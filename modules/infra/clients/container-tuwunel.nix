{ den, ... }:
let
  name = "Tuwunel Container";
  mac = "62:f5:a4:5b:20:91";
in
{
  den.aspects.infra._.clients._.container-tuwunel = {
    aspect-name = "infra.clients.container-tuwunel";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
    ];

    os =
      { config, ... }:
      {
        infra.clients.container-tuwunel = {
          inherit name mac;
          ip = config.infra.vlans.services.staticAddresses.tuwunel;
          ports = {
            default = 6167;
          };
          proxy.port = "default";
        };
      };

    terranix =
      { config, ... }:
      {
        infra.clients.container-tuwunel = {
          inherit name mac;
          ip = config.infra.vlans.services.staticAddresses.tuwunel;
          ports = {
            default = 6167;
          };
          proxy.port = "default";
        };
        resource.unifi_client.container_tuwunel = {
          inherit mac name;
          fixed_ip = config.infra.vlans.services.staticAddresses.tuwunel;
          allow_existing = true;
          blocked = false;
          site = "default";
          skip_forget_on_destroy = false;
        };
        import = [
          {
            to = "unifi_client.container_tuwunel";
            id = mac;
          }
        ];
      };
  };
}
