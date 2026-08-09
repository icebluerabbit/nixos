# home network
{ den, ... }:
let
  name = "qBittorrent Container";
  mac = "62:f5:a4:5b:20:85";
in
{
  den.aspects.infra._.clients._.container-qbittorrent = {
    aspect-name = "infra.clients.container-qbittorrent";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
    ];

    os =
      { config, ... }:
      let
        ip = config.infra.vlans.download.staticAddresses.qbittorrent;
      in
      {
        infra.clients.container-qbittorrent = {
          inherit
            name
            mac
            ip
            ;
          ports = {
            webui = 8080;
            exporter = 8090;
          };
          proxy.port = "webui";
        };
      };

    terranix =
      { config, ... }:
      let
        ip = config.infra.vlans.download.staticAddresses.qbittorrent;
      in
      {
        infra.clients.container-qbittorrent = {
          inherit
            name
            mac
            ip
            ;
          ports = {
            webui = 8080;
            exporter = 8090;
          };
        };

        resource.unifi_client.container_qbittorrent = {
          inherit mac name;
          fixed_ip = ip;
          allow_existing = true;
          blocked = false;
          site = "default";
          skip_forget_on_destroy = false;
        };

        import = [
          {
            to = "unifi_client.container_qbittorrent";
            id = mac;
          }
        ];
      };
  };
}
