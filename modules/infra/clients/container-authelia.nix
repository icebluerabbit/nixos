# authelia — forward authentication service
{ den, ... }:
let
  name = "Authelia Container";
  mac = "62:f5:a4:5b:20:8f";
in
{
  den.aspects.infra._.clients._.container-authelia = {
    aspect-name = "infra.clients.container-authelia";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
    ];

    os =
      { config, ... }:
      let
        ip = config.infra.vlans.services.staticAddresses.authelia;
      in
      {
        infra.clients.container-authelia = {
          inherit name mac ip;
          ports = {
            default = 9091;
            metrics = 9959;
          };
          proxy.port = "default";
        };
      };

    terranix =
      { config, ... }:
      let
        ip = config.infra.vlans.services.staticAddresses.authelia;
      in
      {
        infra.clients.container-authelia = {
          inherit name mac ip;
          ports = {
            default = 9091;
            metrics = 9959;
          };
          proxy.port = "default";
        };

        resource.unifi_client.container_authelia = {
          inherit mac name;
          fixed_ip = ip;
          allow_existing = true;
          blocked = false;
          site = "default";
          skip_forget_on_destroy = false;
        };

        import = [
          {
            to = "unifi_client.container_authelia";
            id = mac;
          }
        ];
      };
  };
}
