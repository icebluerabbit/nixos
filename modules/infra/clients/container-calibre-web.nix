# modules/infra/clients/container-calibre-web.nix
{ den, ... }:
let
  name = "Calibre-Web Container";
  mac = "62:f5:a4:5b:20:86";
in
{
  den.aspects.infra._.clients._.container-calibre-web = {
    aspect-name = "infra.clients.container-calibre-web";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
    ];

    os =
      { config, ... }:
      let
        ip = config.infra.vlans.media.staticAddresses.calibre-web;
      in
      {
        infra.clients.container-calibre-web = {
          inherit
            name
            mac
            ip
            ;
          ports = {
            default = 8083;
          };
          proxy.port = "default";
        };
      };

    terranix =
      { config, ... }:
      let
        ip = config.infra.vlans.media.staticAddresses.calibre-web;
      in
      {
        infra.clients.container-calibre-web = {
          inherit
            name
            mac
            ip
            ;
          ports = {
            default = 8083;
          };
          proxy.port = "default";
        };

        resource.unifi_client.container_calibre_web = {
          inherit mac name;
          fixed_ip = ip;
          allow_existing = true;
          blocked = false;
          site = "default";
          skip_forget_on_destroy = false;
        };

        # import = [
        #   {
        #     to = "unifi_client.container_calibre_web";
        #     id = mac;
        #   }
        # ];
      };
  };
}
