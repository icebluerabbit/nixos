# modules/infra/clients/container-hermes.nix
{ den, ... }:
let
  name = "Hermes Agent Container";
  mac = "62:f5:a4:5b:20:99";
in
{
  den.aspects.infra._.clients._.container-hermes = {
    aspect-name = "infra.clients.container-hermes";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
    ];

    os =
      { config, ... }:
      let
        ip = config.infra.vlans.services.staticAddresses.hermes;
      in
      {
        infra.clients.container-hermes = {
          inherit
            name
            mac
            ip
            ;
          ports = {
            default = 8642;
          };
          proxy.port = "default";
        };
      };

    terranix =
      { config, ... }:
      let
        ip = config.infra.vlans.services.staticAddresses.hermes;
      in
      {
        infra.clients.container-hermes = {
          inherit
            name
            mac
            ip
            ;
          ports = {
            default = 8642;
          };
          proxy.port = "default";
        };

        resource.unifi_client.container_hermes = {
          inherit mac name;
          fixed_ip = ip;
          allow_existing = true;
          blocked = false;
          site = "default";
          skip_forget_on_destroy = false;
        };

        import = [
          {
            to = "unifi_client.container_hermes";
            id = mac;
          }
        ];
      };
  };
}
