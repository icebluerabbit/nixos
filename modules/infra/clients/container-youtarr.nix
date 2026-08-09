# modules/infra/clients/container-youtarr.nix
{ den, ... }:
let
  name = "Youtarr Container";
  mac = "62:f5:a4:5b:20:7e";
in
{
  den.aspects.infra._.clients._.container-youtarr = {
    aspect-name = "infra.clients.container-youtarr";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
    ];

    os =
      { config, ... }:
      {
        infra.clients.container-youtarr = {
          inherit name mac;
          ip = config.infra.vlans.media.staticAddresses.youtarr;
          ports = {
            http = 3011;
          };
          proxy.port = "http";
        };
      };

    terranix =
      { config, ... }:
      {
        infra.clients.container-youtarr = {
          inherit name mac;
          ip = config.infra.vlans.media.staticAddresses.youtarr;
          ports = {
            http = 3011;
          };
          proxy.port = "http";
        };

        resource.unifi_client.container_youtarr = {
          inherit mac name;
          fixed_ip = config.infra.vlans.media.staticAddresses.youtarr;
          allow_existing = true;
          blocked = false;
          site = "default";
        };

        import = [
          {
            to = "unifi_client.container_youtarr";
            id = mac;
          }
        ];
      };
  };
}
