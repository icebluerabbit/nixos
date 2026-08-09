# modules/infra/clients/container-thelounge.nix
{ den, ... }:
let
  name = "TheLounge Container";
  mac = "62:f5:a4:5b:20:75";
in
{
  den.aspects.infra._.clients._.container-thelounge = {
    aspect-name = "infra.clients.container-thelounge";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
    ];

    os =
      { config, ... }:
      let
        ip = config.infra.vlans.apps.staticAddresses.thelounge;
      in
      {
        infra.clients.container-thelounge = {
          inherit
            name
            mac
            ip
            ;
          ports = {
            default = 9000;
          };
          proxy.port = "default";
        };
      };

    terranix =
      { config, ... }:
      let
        ip = config.infra.vlans.apps.staticAddresses.thelounge;
      in
      {
        infra.clients.container-thelounge = {
          inherit
            name
            mac
            ip
            ;
          ports = {
            default = 9000;
          };
          proxy.port = "default";
        };

        resource.unifi_client.container_thelounge = {
          inherit mac name;
          fixed_ip = ip;
          allow_existing = true;
          blocked = false;
          site = "default";
          skip_forget_on_destroy = false;
        };

        import = [
          {
            to = "unifi_client.container_thelounge";
            id = mac;
          }
        ];
      };
  };
}
