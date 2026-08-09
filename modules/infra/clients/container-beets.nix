# modules/infra/clients/container-beets.nix
{ den, ... }:
let
  name = "Beets Container";
  mac = "62:f5:a4:5b:20:87";
in
{
  den.aspects.infra._.clients._.container-beets = {
    aspect-name = "infra.clients.container-beets";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
    ];

    os =
      { config, ... }:
      {
        infra.clients.container-beets = {
          inherit name mac;
          ip = config.infra.vlans.media.staticAddresses.beets;
          ports.default = 8337;
          proxy.port = "default";
        };
      };

    terranix =
      { config, ... }:
      let
        ip = config.infra.vlans.media.staticAddresses.beets;
      in
      {
        infra.clients.container-beets = {
          inherit name mac ip;
          ports.default = 8337;
          proxy.port = "default";
        };

        resource.unifi_client.container_beets = {
          inherit mac name;
          fixed_ip = ip;
          allow_existing = true;
          blocked = false;
          site = "default";
          skip_forget_on_destroy = false;
        };

        import = [
          {
            to = "unifi_client.container_beets";
            id = mac;
          }
        ];
      };
  };
}
