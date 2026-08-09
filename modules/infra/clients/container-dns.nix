# home network
{ den, ... }:
let
  name = "DNS Container";
  mac = "62:f5:a4:5b:20:ad";
in
{
  den.aspects.infra._.clients._.container-dns = {
    aspect-name = "infra.clients.container-dns";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
    ];

    os =
      { config, ... }:
      let
        ip = config.infra.vlans.services.staticAddresses.dns;
      in
      {
        infra.clients.container-dns = {
          inherit
            name
            mac
            ip
            ;
          ports = {
            dns = 53;
            exporter = 9153;
          };
        };
      };

    terranix =
      { config, ... }:
      let
        ip = config.infra.vlans.services.staticAddresses.dns;
      in
      {
        infra.clients.container-dns = {
          inherit
            name
            mac
            ip
            ;
          ports = {
            dns = 53;
            exporter = 9153;
          };
        };

        resource.unifi_client.container_dns = {
          inherit mac name;
          fixed_ip = ip;
          allow_existing = true;
          blocked = false;
          site = "default";
          skip_forget_on_destroy = false;
        };

        import = [
          {
            to = "unifi_client.container_dns";
            id = mac;
          }
        ];
      };
  };
}
