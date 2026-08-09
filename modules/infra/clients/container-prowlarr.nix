# modules/infra/clients/container-prowlarr.nix
{ den, ... }:
let
  name = "Prowlarr Container";
  mac = "62:f5:a4:5b:20:7d";
in
{
  den.aspects.infra._.clients._.container-prowlarr = {
    aspect-name = "infra.clients.container-prowlarr";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
    ];

    os =
      { config, ... }:
      let
        ip = config.infra.vlans.media.staticAddresses.prowlarr;
      in
      {
        infra.clients.container-prowlarr = {
          inherit
            name
            mac
            ip
            ;
          ports = {
            default = 9696;
            exporter = 9707;
          };
          proxy.port = "default";
        };
      };

    terranix =
      { config, ... }:
      let
        ip = config.infra.vlans.media.staticAddresses.prowlarr;
      in
      {
        infra.clients.container-prowlarr = {
          inherit
            name
            mac
            ip
            ;
          ports = {
            default = 9696;
            exporter = 9707;
          };
          proxy.port = "default";
        };

        resource.unifi_client.container_prowlarr = {
          inherit mac name;
          fixed_ip = ip;
          allow_existing = true;
          blocked = false;
          site = "default";
          skip_forget_on_destroy = false;
        };

        import = [
          {
            to = "unifi_client.container_prowlarr";
            id = mac;
          }
        ];
      };
  };
}
