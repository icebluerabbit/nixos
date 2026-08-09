# home network
{ den, ... }:
let
  name = "FlareSolverr Container";
  mac = "62:f5:a4:5b:20:83";
in
{
  den.aspects.infra._.clients._.container-flaresolverr = {
    aspect-name = "infra.clients.container-flaresolverr";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
    ];

    os =
      { config, ... }:
      let
        ip = config.infra.vlans.media.staticAddresses.flaresolverr;
      in
      {
        infra.clients.container-flaresolverr = {
          inherit
            name
            mac
            ip
            ;
          ports = {
            default = 8191;
          };
        };
      };

    terranix =
      { config, ... }:
      let
        ip = config.infra.vlans.media.staticAddresses.flaresolverr;
      in
      {
        infra.clients.container-flaresolverr = {
          inherit
            name
            mac
            ip
            ;
          ports = {
            default = 8191;
          };
        };

        resource.unifi_client.container_flaresolverr = {
          inherit mac name;
          fixed_ip = ip;
          allow_existing = true;
          blocked = false;
          site = "default";
          skip_forget_on_destroy = false;
        };

        import = [
          {
            to = "unifi_client.container_flaresolverr";
            id = mac;
          }
        ];
      };
  };
}
