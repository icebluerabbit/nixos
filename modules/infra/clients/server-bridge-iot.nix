# home network
{ den, ... }:
let
  name = "Server IoT Bridge";
  mac = "82:13:8a:a0:4f:3e";
in
{
  den.aspects.infra._.clients._.server-bridge-iot = {
    aspect-name = "infra.clients.server-bridge-iot";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options

    ];

    os =
      { config, ... }:
      let
        ip = config.infra.vlans.iot.staticAddresses.server;
      in
      {
        infra.clients.server-bridge-iot = {
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
        ip = config.infra.vlans.iot.staticAddresses.server;
      in
      {
        infra.clients.server-bridge-iot = {
          inherit
            name
            mac
            ip
            ;
        };

        resource.unifi_client.server_bridge_iot = {
          inherit mac name;
          fixed_ip = ip;
          allow_existing = true;
          blocked = false;
          site = "default";
          skip_forget_on_destroy = false;
          depends_on = [ "unifi_client.server_vlan_iot" ];
        };

        import = [
          {
            to = "unifi_client.server_bridge_iot";
            id = mac;
          }
        ];
      };
  };
}
