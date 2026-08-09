# modules/infra/clients/container-loki.nix
{ den, ... }:
let
  name = "Loki Container";
  mac = "62:f5:a4:5b:20:94";
in
{
  den.aspects.infra._.clients._.container-loki = {
    aspect-name = "infra.clients.container-loki";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
    ];

    os =
      { config, ... }:
      let
        ip = config.infra.vlans.services.staticAddresses.loki;
      in
      {
        infra.clients.container-loki = {
          inherit name mac ip;
          ports = {
            default = 3100;
          };
        };
      };

    terranix =
      { config, ... }:
      let
        ip = config.infra.vlans.services.staticAddresses.loki;
      in
      {
        infra.clients.container-loki = {
          inherit name mac ip;
          ports = {
            default = 3100;
          };
        };

        resource.unifi_client.container_loki = {
          inherit mac name;
          fixed_ip = ip;
          allow_existing = true;
          blocked = false;
          site = "default";
          skip_forget_on_destroy = false;
        };

        import = [
          {
            to = "unifi_client.container_loki";
            id = mac;
          }
        ];
      };
  };
}
