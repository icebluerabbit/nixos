# home network
{ den, ... }:
let
  name = "Server";
  mac = "9c:6b:00:d2:18:a6";
in
{
  den.aspects.infra._.clients._.server = {
    aspect-name = "infra.clients.server";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options

    ];

    os =
      { config, ... }:
      let
        ip = config.infra.vlans.mgmt.staticAddresses.server;
      in
      {
        infra.clients.server = {
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
        ip = config.infra.vlans.mgmt.staticAddresses.server;
      in
      {
        infra.clients.server = {
          inherit
            name
            mac
            ip
            ;
        };

        resource.unifi_client.server = {
          inherit mac name;
          fixed_ip = ip;
          allow_existing = true;
          blocked = false;
          site = "default";
          skip_forget_on_destroy = false;
        };

        import = [
          {
            to = "unifi_client.server";
            id = mac;
          }
        ];
      };
  };
}
