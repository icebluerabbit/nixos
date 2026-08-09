# home network
{ den, ... }:
let
  name = "Autobrr Container";
  mac = "62:f5:a4:5b:20:b2";
in
{
  den.aspects.infra._.clients._.container-autobrr = {
    aspect-name = "infra.clients.container-autobrr";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
    ];

    os =
      { config, ... }:
      let
        ip = config.infra.vlans.media.staticAddresses.autobrr;
      in
      {
        infra.clients.container-autobrr = {
          inherit
            name
            mac
            ip
            ;
          ports = {
            default = 7474;
          };
          proxy.port = "default";
        };
      };

    terranix =
      { config, ... }:
      let
        ip = config.infra.vlans.media.staticAddresses.autobrr;
      in
      {
        infra.clients.container-autobrr = {
          inherit
            name
            mac
            ip
            ;
          ports = {
            default = 7474;
          };
        };

        resource.unifi_client.container_autobrr = {
          inherit mac name;
          fixed_ip = ip;
          allow_existing = true;
          blocked = false;
          site = "default";
          skip_forget_on_destroy = false;
        };

        import = [
          {
            to = "unifi_client.container_autobrr";
            id = mac;
          }
        ];
      };
  };
}
