# home network
{ den, ... }:
let
  name = "UNAS-Pro";
  mac = "6c:63:f8:10:41:a2";
in
{
  den.aspects.infra._.clients._.unas-pro = {
    aspect-name = "infra.clients.unas-pro";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
      den.aspects.infra._.networks._.storage
    ];

    os =
      { config, ... }:
      {
        infra.clients.unas-pro = {
          inherit name mac;
          ip = config.infra.vlans.storage.staticAddresses.unas-pro;
        };
      };

    terranix =
      { config, ... }:
      let
        ip = config.infra.vlans.storage.staticAddresses.unas-pro;
      in
      {
        infra.clients.unas-pro = {
          inherit name mac ip;
        };

        resource.unifi_client.unas_pro = {
          inherit mac name;
          fixed_ip = ip;
          allow_existing = true;
          blocked = false;
          site = "default";
          skip_forget_on_destroy = false;
        };

        import = [
          {
            to = "unifi_client.unas_pro";
            id = mac;
          }
        ];
      };
  };
}
