# modules/infra/clients/container-openchamber.nix
{ den, ... }:
let
  name = "OpenChamber Container";
  mac = "62:f5:a4:5b:20:76";
in
{
  den.aspects.infra._.clients._.container-openchamber = {
    aspect-name = "infra.clients.container-openchamber";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
    ];

    os =
      { config, ... }:
      let
        ip = config.infra.vlans.apps.staticAddresses.openchamber;
      in
      {
        infra.clients.container-openchamber = {
          inherit
            name
            mac
            ip
            ;
          ports = {
            default = 3000;
          };
          proxy.port = "default";
        };
      };

    terranix =
      { config, ... }:
      let
        ip = config.infra.vlans.apps.staticAddresses.openchamber;
      in
      {
        infra.clients.container-openchamber = {
          inherit
            name
            mac
            ip
            ;
          ports = {
            default = 3000;
          };
          proxy.port = "default";
        };

        resource.unifi_client.container_openchamber = {
          inherit mac name;
          fixed_ip = ip;
          allow_existing = true;
          blocked = false;
          site = "default";
          skip_forget_on_destroy = false;
        };

        import = [
          {
            to = "unifi_client.container_openchamber";
            id = mac;
          }
        ];
      };
  };
}
