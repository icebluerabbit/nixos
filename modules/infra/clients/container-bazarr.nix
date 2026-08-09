# modules/infra/clients/container-bazarr.nix
{ den, ... }:
let
  name = "Bazarr Container";
  # index 60 (0xb4) — media's deterministic index range (vlanId 120 + idx)
  # overflows into other VLANs' byte range past idx ~9, so overflow media
  # containers (flaresolverr, autobrr, cross-seed) jump to a high free byte.
  mac = "62:f5:a4:5b:20:b4";
in
{
  den.aspects.infra._.clients._.container-bazarr = {
    aspect-name = "infra.clients.container-bazarr";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
    ];

    os =
      { config, ... }:
      let
        ip = config.infra.vlans.media.staticAddresses.bazarr;
      in
      {
        infra.clients.container-bazarr = {
          inherit
            name
            mac
            ip
            ;
          ports = {
            default = 6767;
          };
          proxy.port = "default";
        };
      };

    terranix =
      { config, ... }:
      let
        ip = config.infra.vlans.media.staticAddresses.bazarr;
      in
      {
        infra.clients.container-bazarr = {
          inherit
            name
            mac
            ip
            ;
          ports = {
            default = 6767;
          };
          proxy.port = "default";
        };

        resource.unifi_client.container_bazarr = {
          inherit mac name;
          fixed_ip = ip;
          allow_existing = true;
          blocked = false;
          site = "default";
          skip_forget_on_destroy = false;
        };

        import = [
          {
            to = "unifi_client.container_bazarr";
            id = mac;
          }
        ];
      };
  };
}
