# modules/infra/clients/container-home-assistant.nix
{ den, ... }:
let
  name = "Home Assistant Container";
  mac = "62:f5:a4:5b:20:1f";
in
{
  den.aspects.infra._.clients._.container-home-assistant = {
    aspect-name = "infra.clients.container-home-assistant";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
    ];

    os =
      { config, ... }:
      let
        ip = config.infra.vlans.iot.staticAddresses.home-assistant;
      in
      {
        infra.clients.container-home-assistant = {
          inherit
            name
            mac
            ip
            ;
          ports = {
            http = 8123;
          };
          proxy.port = "http";
        };
      };

    terranix =
      { config, ... }:
      let
        ip = config.infra.vlans.iot.staticAddresses.home-assistant;
      in
      {
        infra.clients.container-home-assistant = {
          inherit
            name
            mac
            ip
            ;
          ports = {
            http = 8123;
          };
          proxy.port = "http";
        };

        resource.unifi_client.container_home_assistant = {
          inherit mac name;
          fixed_ip = ip;
          allow_existing = true;
          blocked = false;
          site = "default";
          skip_forget_on_destroy = false;
        };

        import = [
          {
            to = "unifi_client.container_home_assistant";
            id = mac;
          }
        ];
      };
  };
}
