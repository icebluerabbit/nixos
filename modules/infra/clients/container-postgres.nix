# modules/infra/clients/container-postgres.nix
{ den, ... }:
let
  name = "PostgreSQL Container";
  mac = "62:f5:a4:5b:20:8d";
in
{
  den.aspects.infra._.clients._.container-postgres = {
    aspect-name = "infra.clients.container-postgres";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options

    ];

    os =
      { config, ... }:
      let
        ip = config.infra.vlans.services.staticAddresses.postgres;
      in
      {
        infra.clients.container-postgres = {
          inherit
            name
            mac
            ip
            ;
          ports = {
            default = 5432;
            exporter = 9187;
          };
        };
      };

    terranix =
      { config, ... }:
      let
        ip = config.infra.vlans.services.staticAddresses.postgres;
      in
      {
        infra.clients.container-postgres = {
          inherit
            name
            mac
            ip
            ;
          ports = {
            default = 5432;
            exporter = 9187;
          };
        };

        resource.unifi_client.container_postgres = {
          inherit mac name;
          fixed_ip = ip;
          allow_existing = true;
          blocked = false;
          site = "default";
          skip_forget_on_destroy = false;
        };

        import = [
          {
            to = "unifi_client.container_postgres";
            id = mac;
          }
        ];
      };
  };
}
