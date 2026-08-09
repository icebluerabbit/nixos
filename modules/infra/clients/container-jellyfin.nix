# modules/infra/clients/container-jellyfin.nix
{ den, ... }:
let
  name = "Jellyfin Container";
  mac = "62:f5:a4:5b:20:7f";
in
{
  den.aspects.infra._.clients._.container-jellyfin = {
    aspect-name = "infra.clients.container-jellyfin";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
    ];

    os =
      { config, ... }:
      let
        ip = config.infra.vlans.media.staticAddresses.jellyfin;
      in
      {
        infra.clients.container-jellyfin = {
          inherit
            name
            mac
            ip
            ;
          ports = {
            http = 8096;
          };
          proxy.port = "http";
        };
      };

    terranix =
      { config, ... }:
      let
        ip = config.infra.vlans.media.staticAddresses.jellyfin;
      in
      {
        infra.clients.container-jellyfin = {
          inherit
            name
            mac
            ip
            ;
          ports = {
            http = 8096;
          };
          proxy.port = "http";
        };

        resource.unifi_client.container_jellyfin = {
          inherit mac name;
          fixed_ip = ip;
          allow_existing = true;
          blocked = false;
          site = "default";
          skip_forget_on_destroy = false;
        };

        import = [
          {
            to = "unifi_client.container_jellyfin";
            id = mac;
          }
        ];
      };
  };
}
