# home network
{ den, ... }:
let
  name = "Server IPMI";
  mac = "9c:6b:00:d2:25:5f";
in
{
  den.aspects.infra._.clients._.server-ipmi = {
    aspect-name = "infra.clients.server-ipmi";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options

    ];

    os =
      { config, ... }:
      let
        ip = config.infra.vlans.mgmt.staticAddresses.server-ipmi;
      in
      {
        infra.clients.server-ipmi = {
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
        ip = config.infra.vlans.mgmt.staticAddresses.server-ipmi;
      in
      {
        infra.clients.server-ipmi = {
          inherit
            name
            mac
            ip
            ;
        };

        resource.unifi_client.server_ipmi = {
          inherit mac name;
          fixed_ip = ip;
          allow_existing = true;
          blocked = false;
          site = "default";
          skip_forget_on_destroy = false;
        };

        import = [
          {
            to = "unifi_client.server_ipmi";
            id = mac;
          }
        ];
      };
  };
}
