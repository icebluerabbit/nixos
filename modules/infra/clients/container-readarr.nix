# modules/infra/clients/container-readarr.nix
{ den, ... }:
let
  name = "Readarr Container";
  mac = "62:f5:a4:5b:20:7b";
in
{
  den.aspects.infra._.clients._.container-readarr = {
    aspect-name = "infra.clients.container-readarr";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
    ];

    os =
      { config, ... }:
      let
        ip = config.infra.vlans.media.staticAddresses.readarr;
      in
      {
        infra.clients.container-readarr = {
          inherit
            name
            mac
            ip
            ;
          ports = {
            default = 8787;
          };
          proxy.port = "default";
        };
      };

    terranix =
      { config, ... }:
      let
        ip = config.infra.vlans.media.staticAddresses.readarr;
      in
      {
        infra.clients.container-readarr = {
          inherit
            name
            mac
            ip
            ;
          ports = {
            default = 8787;
          };
          proxy.port = "default";
        };

        resource.unifi_client.container_readarr = {
          inherit mac name;
          fixed_ip = ip;
          allow_existing = true;
          blocked = false;
          site = "default";
          skip_forget_on_destroy = false;
        };

        import = [
          {
            to = "unifi_client.container_readarr";
            id = mac;
          }
        ];
      };
  };
}
