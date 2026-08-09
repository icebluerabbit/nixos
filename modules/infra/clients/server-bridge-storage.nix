# home network
{ den, ... }:
let
  name = "Server Storage Bridge";
  mac = "82:13:8a:a0:4f:52";
in
{
  den.aspects.infra._.clients._.server-bridge-storage = {
    aspect-name = "infra.clients.server-bridge-storage";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options

    ];

    os =
      { config, ... }:
      let
        ip = config.infra.vlans.storage.staticAddresses.server;
      in
      {
        infra.clients.server-bridge-storage = {
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
        ip = config.infra.vlans.storage.staticAddresses.server;
      in
      {
        infra.clients.server-bridge-storage = {
          inherit
            name
            mac
            ip
            ;
        };

        resource.unifi_client.server_bridge_storage = {
          inherit mac name;
          fixed_ip = ip;
          allow_existing = true;
          blocked = false;
          site = "default";
          skip_forget_on_destroy = false;
          depends_on = [ "unifi_client.server_vlan_storage" ];
        };

        import = [
          {
            to = "unifi_client.server_bridge_storage";
            id = mac;
          }
        ];
      };
  };
}
