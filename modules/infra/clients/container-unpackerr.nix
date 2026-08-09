# modules/infra/clients/container-unpackerr.nix
{ den, ... }:
let
  name = "Unpackerr Container";
  mac = "62:f5:a4:5b:20:81";
in
{
  den.aspects.infra._.clients._.container-unpackerr = {
    aspect-name = "infra.clients.container-unpackerr";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options

    ];

    os =
      { config, ... }:
      let
        ip = config.infra.vlans.media.staticAddresses.unpackerr;
      in
      {
        infra.clients.container-unpackerr = {
          inherit
            name
            mac
            ip
            ;
          # Only listener is the built-in Prometheus metrics webserver.
          ports = {
            metrics = 5656;
          };
        };
      };

    terranix =
      { config, ... }:
      let
        ip = config.infra.vlans.media.staticAddresses.unpackerr;
      in
      {
        infra.clients.container-unpackerr = {
          inherit
            name
            mac
            ip
            ;
          ports = { };
        };

        resource.unifi_client.container_unpackerr = {
          inherit mac name;
          fixed_ip = ip;
          allow_existing = true;
          blocked = false;
          site = "default";
          skip_forget_on_destroy = false;
        };

        import = [
          {
            to = "unifi_client.container_unpackerr";
            id = mac;
          }
        ];
      };
  };
}
