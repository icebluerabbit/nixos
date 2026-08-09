# modules/infra/clients/container-pgadmin.nix
{ den, ... }:
let
  name = "pgAdmin Container";
  mac = "62:f5:a4:5b:20:8e";
in
{
  den.aspects.infra._.clients._.container-pgadmin = {
    aspect-name = "infra.clients.container-pgadmin";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options

    ];

    os =
      { config, ... }:
      let
        ip = config.infra.vlans.services.staticAddresses.pgadmin;
      in
      {
        infra.clients.container-pgadmin = {
          inherit
            name
            mac
            ip
            ;
          ports = {
            default = 5050;
          };
          proxy.port = "default";
        };
      };

    terranix =
      { config, ... }:
      let
        ip = config.infra.vlans.services.staticAddresses.pgadmin;
      in
      {
        infra.clients.container-pgadmin = {
          inherit
            name
            mac
            ip
            ;
          ports = {
            default = 5050;
          };
          proxy.port = "default";
        };

        resource.unifi_client.container_pgadmin = {
          inherit mac name;
          fixed_ip = ip;
          allow_existing = true;
          blocked = false;
          site = "default";
          skip_forget_on_destroy = false;
        };

        import = [
          {
            to = "unifi_client.container_pgadmin";
            id = mac;
          }
        ];
      };
  };
}
