# modules/infra/clients/container-lazylibrarian.nix
{ den, ... }:
let
  name = "LazyLibrarian Container";
  mac = "62:f5:a4:5b:20:80";
in
{
  den.aspects.infra._.clients._.container-lazylibrarian = {
    aspect-name = "infra.clients.container-lazylibrarian";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
    ];

    os =
      { config, ... }:
      let
        ip = config.infra.vlans.media.staticAddresses.lazylibrarian;
      in
      {
        infra.clients.container-lazylibrarian = {
          inherit
            name
            mac
            ip
            ;
          ports = {
            default = 5299;
          };
          proxy.port = "default";
        };
      };

    terranix =
      { config, ... }:
      let
        ip = config.infra.vlans.media.staticAddresses.lazylibrarian;
      in
      {
        infra.clients.container-lazylibrarian = {
          inherit
            name
            mac
            ip
            ;
          ports = {
            default = 5299;
          };
          proxy.port = "default";
        };

        resource.unifi_client.container_lazylibrarian = {
          inherit mac name;
          fixed_ip = ip;
          allow_existing = true;
          blocked = false;
          site = "default";
          skip_forget_on_destroy = false;
        };

        import = [
          {
            to = "unifi_client.container_lazylibrarian";
            id = mac;
          }
        ];
      };
  };
}
