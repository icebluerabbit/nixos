# modules/infra/clients/container-soularr.nix
{ den, ... }:
let
  name = "Soularr Container";
  mac = "62:f5:a4:5b:20:89"; # globally-unique last byte (:87 beets, :88 slskd)
in
{
  den.aspects.infra._.clients._.container-soularr = {
    aspect-name = "infra.clients.container-soularr";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
    ];

    os =
      { config, ... }:
      let
        ip = config.infra.vlans.media.staticAddresses.soularr;
      in
      {
        infra.clients.container-soularr = {
          inherit
            name
            mac
            ip
            ;
          ports = {
            webui = 8265;
          };
          proxy.port = "webui";
        };
      };

    terranix =
      { config, ... }:
      let
        ip = config.infra.vlans.media.staticAddresses.soularr;
      in
      {
        infra.clients.container-soularr = {
          inherit
            name
            mac
            ip
            ;
          ports = {
            webui = 8265;
          };
          proxy.port = "webui";
        };

        resource.unifi_client.container_soularr = {
          inherit mac name;
          fixed_ip = ip;
          allow_existing = true;
          blocked = false;
          site = "default";
          skip_forget_on_destroy = false;
        };

        import = [
          {
            to = "unifi_client.container_soularr";
            id = mac;
          }
        ]; # uncomment AFTER first apply
      };
  };
}
