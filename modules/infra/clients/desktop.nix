# home network
{ den, ... }:
let
  name = "Desktop";
  mac = "b0:82:e2:32:f9:94";
in
{
  den.aspects.infra._.clients._.desktop = {
    aspect-name = "infra.clients.desktop";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
    ];

    os =
      { config, ... }:
      let
        ip = config.infra.vlans.trust.staticAddresses.desktop;
      in
      {
        infra.clients.desktop = {
          inherit name mac ip;
        };
      };

    terranix =
      { config, ... }:
      let
        ip = config.infra.vlans.trust.staticAddresses.desktop;
      in
      {
        infra.clients.desktop = {
          inherit name mac ip;
        };

        resource.unifi_client.desktop = {
          inherit mac name;
          fixed_ip = ip;
          allow_existing = true;
          blocked = false;
          site = "default";
          skip_forget_on_destroy = false;
        };

        import = [
          {
            to = "unifi_client.desktop";
            id = mac;
          }
        ];
      };
  };
}
