# home network
{ den, ... }:
let
  name = "reTerminal E1003";
  mac = "44:1b:f6:c0:04:54";
in
{
  den.aspects.infra._.clients._.reterminal-e1003 = {
    aspect-name = "infra.clients.reterminal-e1003";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
    ];

    os =
      { config, ... }:
      let
        ip = config.infra.vlans.iot.staticAddresses.reterminal-e1003;
      in
      {
        infra.clients.reterminal-e1003 = {
          inherit name mac ip;
        };
      };

    terranix =
      { config, ... }:
      let
        ip = config.infra.vlans.iot.staticAddresses.reterminal-e1003;
      in
      {
        infra.clients.reterminal-e1003 = {
          inherit name mac ip;
        };

        resource.unifi_client.reterminal_e1003 = {
          inherit mac name;
          fixed_ip = ip;
          allow_existing = true;
          blocked = false;
          site = "default";
          skip_forget_on_destroy = false;
        };

        import = [
          {
            to = "unifi_client.reterminal_e1003";
            id = mac;
          }
        ];
      };
  };
}
