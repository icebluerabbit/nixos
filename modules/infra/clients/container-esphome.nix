# modules/infra/clients/container-esphome.nix
{ den, ... }:
let
  name = "ESPHome Container";
  mac = "62:f5:a4:5b:20:20";
in
{
  den.aspects.infra._.clients._.container-esphome = {
    aspect-name = "infra.clients.container-esphome";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
    ];

    os =
      { config, ... }:
      let
        ip = config.infra.vlans.iot.staticAddresses.esphome;
      in
      {
        infra.clients.container-esphome = {
          inherit
            name
            mac
            ip
            ;
        };
      };

    terranix =
      { config, ... }:
      let
        ip = config.infra.vlans.iot.staticAddresses.esphome;
      in
      {
        infra.clients.container-esphome = {
          inherit
            name
            mac
            ip
            ;
        };

        resource.unifi_client.container_esphome = {
          inherit mac name;
          fixed_ip = ip;
          allow_existing = true;
          blocked = false;
          site = "default";
          skip_forget_on_destroy = false;
        };

        import = [
          {
            to = "unifi_client.container_esphome";
            id = mac;
          }
        ];
      };
  };
}
