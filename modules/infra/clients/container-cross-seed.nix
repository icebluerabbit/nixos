# home network
{ den, ... }:
let
  name = "cross-seed Container";
  mac = "62:f5:a4:5b:20:b3";
in
{
  den.aspects.infra._.clients._.container-cross-seed = {
    aspect-name = "infra.clients.container-cross-seed";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
    ];

    os =
      { config, ... }:
      let
        ip = config.infra.vlans.media.staticAddresses.cross-seed;
      in
      {
        infra.clients.container-cross-seed = {
          inherit
            name
            mac
            ip
            ;
          ports = {
            default = 2468;
          };
          proxy.port = "default";
        };
      };

    terranix =
      { config, ... }:
      let
        ip = config.infra.vlans.media.staticAddresses.cross-seed;
      in
      {
        infra.clients.container-cross-seed = {
          inherit
            name
            mac
            ip
            ;
          ports = {
            default = 2468;
          };
          proxy.port = "default";
        };

        resource.unifi_client.container_cross_seed = {
          inherit mac name;
          fixed_ip = ip;
          allow_existing = true;
          blocked = false;
          site = "default";
          skip_forget_on_destroy = false;
        };

        import = [
          {
            to = "unifi_client.container_cross_seed";
            id = mac;
          }
        ];
      };
  };
}
