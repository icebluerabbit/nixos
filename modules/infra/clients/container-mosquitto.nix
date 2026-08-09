# modules/infra/clients/container-mosquitto.nix
{ den, ... }:
let
  name = "Mosquitto Container";
  mac = "62:f5:a4:5b:20:22";
in
{
  den.aspects.infra._.clients._.container-mosquitto = {
    aspect-name = "infra.clients.container-mosquitto";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
    ];

    os =
      { config, ... }:
      {
        infra.clients.container-mosquitto = {
          inherit name mac;
          ip = config.infra.vlans.iot.staticAddresses.mosquitto;
          ports.default = 1883;
        };
      };

    terranix =
      { config, ... }:
      let
        ip = config.infra.vlans.iot.staticAddresses.mosquitto;
      in
      {
        infra.clients.container-mosquitto = {
          inherit name mac ip;
          ports.default = 1883;
        };

        resource.unifi_client.container_mosquitto = {
          inherit mac name;
          fixed_ip = ip;
          allow_existing = true;
          blocked = false;
          site = "default";
          skip_forget_on_destroy = false;
        };

        import = [
          {
            to = "unifi_client.container_mosquitto";
            id = mac;
          }
        ];
      };
  };
}
