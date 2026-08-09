# modules/infra/clients/container-opencode.nix
{ den, ... }:
let
  name = "OpenCode Container";
  mac = "62:f5:a4:5b:20:93";
in
{
  den.aspects.infra._.clients._.container-opencode = {
    aspect-name = "infra.clients.container-opencode";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
    ];

    os =
      { config, ... }:
      let
        ip = config.infra.vlans.services.staticAddresses.opencode;
      in
      {
        infra.clients.container-opencode = {
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
        ip = config.infra.vlans.services.staticAddresses.opencode;
      in
      {
        infra.clients.container-opencode = {
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

        resource.unifi_client.container_opencode = {
          inherit mac name;
          fixed_ip = ip;
          allow_existing = true;
          blocked = false;
          site = "default";
          skip_forget_on_destroy = false;
        };

        import = [
          {
            to = "unifi_client.container_opencode";
            id = mac;
          }
        ];
      };
  };
}
