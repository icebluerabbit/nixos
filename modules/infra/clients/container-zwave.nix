# modules/infra/clients/container-zwave.nix
{ den, ... }:
let
  name = "Z-Wave Container";
  mac = "62:f5:a4:5b:20:21";
in
{
  den.aspects.infra._.clients._.container-zwave = {
    aspect-name = "infra.clients.container-zwave";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
    ];

    os =
      { config, ... }:
      let
        ip = config.infra.vlans.iot.staticAddresses.zwave;
      in
      {
        infra.clients.container-zwave = {
          inherit name mac ip;
          ports = {
            ui = 8091;
            ws = 3000;
          };
          proxy.port = "ui";
        };
      };

    terranix =
      { config, ... }:
      let
        ip = config.infra.vlans.iot.staticAddresses.zwave;
      in
      {
        infra.clients.container-zwave = {
          inherit name mac ip;
          ports = {
            ui = 8091;
            ws = 3000;
          };
          proxy.port = "ui";
        };

        resource.unifi_client.container_zwave = {
          inherit mac name;
          fixed_ip = ip;
          allow_existing = true;
          blocked = false;
          site = "default";
          skip_forget_on_destroy = false;
        };

        import = [
          {
            to = "unifi_client.container_zwave";
            id = mac;
          }
        ];
      };
  };
}
