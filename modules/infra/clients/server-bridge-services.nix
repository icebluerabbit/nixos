# home network
{ den, ... }:
let
  name = "Server Services Bridge";
  mac = "82:13:8a:a0:4f:ac";
in
{
  den.aspects.infra._.clients._.server-bridge-services = {
    aspect-name = "infra.clients.server-bridge-services";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options

    ];

    os =
      { config, ... }:
      let
        ip = config.infra.vlans.services.staticAddresses.server;
      in
      {
        infra.clients.server-bridge-services = {
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
        ip = config.infra.vlans.services.staticAddresses.server;
      in
      {
        infra.clients.server-bridge-services = {
          inherit
            name
            mac
            ip
            ;
        };

        resource.unifi_client.server_bridge_services = {
          inherit mac name;
          fixed_ip = ip;
          allow_existing = true;
          blocked = false;
          site = "default";
          skip_forget_on_destroy = false;
          depends_on = [ "unifi_client.server_vlan_services" ];
        };

        import = [
          {
            to = "unifi_client.server_bridge_services";
            id = mac;
          }
        ];
      };
  };
}
