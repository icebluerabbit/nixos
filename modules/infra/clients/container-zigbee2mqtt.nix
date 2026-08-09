# modules/infra/clients/container-zigbee2mqtt.nix
{ den, ... }:
let
  name = "Zigbee2MQTT Container";
  mac = "62:f5:a4:5b:20:23";
in
{
  den.aspects.infra._.clients._.container-zigbee2mqtt = {
    aspect-name = "infra.clients.container-zigbee2mqtt";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
    ];

    os =
      { config, ... }:
      {
        infra.clients.container-zigbee2mqtt = {
          inherit name mac;
          ip = config.infra.vlans.iot.staticAddresses.zigbee2mqtt;
          ports.default = 8080;
          proxy.port = "default";
        };
      };

    terranix =
      { config, ... }:
      let
        ip = config.infra.vlans.iot.staticAddresses.zigbee2mqtt;
      in
      {
        infra.clients.container-zigbee2mqtt = {
          inherit name mac ip;
          ports.default = 8080;
          proxy.port = "default";
        };

        resource.unifi_client.container_zigbee2mqtt = {
          inherit mac name;
          fixed_ip = ip;
          allow_existing = true;
          blocked = false;
          site = "default";
          skip_forget_on_destroy = false;
        };

        import = [
          {
            to = "unifi_client.container_zigbee2mqtt";
            id = mac;
          }
        ];
      };
  };
}
