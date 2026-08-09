# modules/infra/clients/container-homepage.nix
{ den, ... }:
let
  name = "Homepage Container";
  mac = "62:f5:a4:5b:20:71";
in
{
  den.aspects.infra._.clients._.container-homepage = {
    aspect-name = "infra.clients.container-homepage";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
    ];

    os =
      { config, ... }:
      let
        ip = config.infra.vlans.apps.staticAddresses.homepage;
      in
      {
        infra.clients.container-homepage = {
          inherit
            name
            mac
            ip
            ;
          ports = {
            default = 8082;
          };
          proxy.port = "default";
        };
      };

    terranix =
      { config, ... }:
      let
        ip = config.infra.vlans.apps.staticAddresses.homepage;
      in
      {
        infra.clients.container-homepage = {
          inherit
            name
            mac
            ip
            ;
          ports = {
            default = 8082;
          };
          proxy.port = "default";
        };

        resource.unifi_client.container_homepage = {
          inherit mac name;
          fixed_ip = ip;
          allow_existing = true;
          blocked = false;
          site = "default";
          skip_forget_on_destroy = false;
        };

        # import = [
        #   {
        #     to = "unifi_client.container_homepage";
        #     id = mac;
        #   }
        # ];
      };
  };
}
