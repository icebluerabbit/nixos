# home network
{ den, ... }:
let
  name = "Server Media Bridge";
  mac = "82:13:8a:a0:4f:98";
in
{
  den.aspects.infra._.clients._.server-bridge-media = {
    aspect-name = "infra.clients.server-bridge-media";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options

    ];

    os =
      { config, ... }:
      let
        ip = config.infra.vlans.media.staticAddresses.server;
      in
      {
        infra.clients.server-bridge-media = {
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
        ip = config.infra.vlans.media.staticAddresses.server;
      in
      {
        infra.clients.server-bridge-media = {
          inherit
            name
            mac
            ip
            ;
        };

        resource.unifi_client.server_bridge_media = {
          inherit mac name;
          fixed_ip = ip;
          allow_existing = true;
          blocked = false;
          site = "default";
          skip_forget_on_destroy = false;
          depends_on = [ "unifi_client.server_vlan_media" ];
        };

        import = [
          {
            to = "unifi_client.server_bridge_media";
            id = mac;
          }
        ];
      };
  };
}
