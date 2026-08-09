# home network
{ den, ... }:
let
  name = "Server DMZ Bridge";
  mac = "82:13:8a:a0:4f:84";
in
{
  den.aspects.infra._.clients._.server-bridge-dmz = {
    aspect-name = "infra.clients.server-bridge-dmz";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options

    ];

    os =
      { config, ... }:
      let
        ip = config.infra.vlans.dmz.staticAddresses.server;
      in
      {
        infra.clients.server-bridge-dmz = {
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
        ip = config.infra.vlans.dmz.staticAddresses.server;
      in
      {
        infra.clients.server-bridge-dmz = {
          inherit
            name
            mac
            ip
            ;
        };

        resource.unifi_client.server_bridge_dmz = {
          inherit mac name;
          fixed_ip = ip;
          allow_existing = true;
          blocked = false;
          site = "default";
          skip_forget_on_destroy = false;
          depends_on = [ "unifi_client.server_vlan_dmz" ];
        };

        import = [
          {
            to = "unifi_client.server_bridge_dmz";
            id = mac;
          }
        ];
      };
  };
}
