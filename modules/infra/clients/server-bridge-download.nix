# home network
{ den, ... }:
let
  name = "Server Download Bridge";
  mac = "82:13:8a:a0:4f:a2";
in
{
  den.aspects.infra._.clients._.server-bridge-download = {
    aspect-name = "infra.clients.server-bridge-download";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options

    ];

    os =
      { config, ... }:
      let
        ip = config.infra.vlans.download.staticAddresses.server;
      in
      {
        infra.clients.server-bridge-download = {
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
        ip = config.infra.vlans.download.staticAddresses.server;
      in
      {
        infra.clients.server-bridge-download = {
          inherit
            name
            mac
            ip
            ;
        };

        resource.unifi_client.server_bridge_download = {
          inherit mac name;
          fixed_ip = ip;
          allow_existing = true;
          blocked = false;
          site = "default";
          skip_forget_on_destroy = false;
          depends_on = [ "unifi_client.server_vlan_download" ];
        };

        import = [
          {
            to = "unifi_client.server_bridge_download";
            id = mac;
          }
        ];
      };
  };
}
