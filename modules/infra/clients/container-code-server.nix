# modules/infra/clients/container-code-server.nix
{ den, ... }:
let
  name = "Code Server Container";
  mac = "62:f5:a4:5b:20:6f"; # apps VLAN, index 1 (next_free_mac)
in
{
  den.aspects.infra._.clients._.container-code-server = {
    aspect-name = "infra.clients.container-code-server";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
    ];

    os =
      { config, ... }:
      let
        ip = config.infra.vlans.apps.staticAddresses.code-server;
      in
      {
        infra.clients.container-code-server = {
          inherit
            name
            mac
            ip
            ;
          ports = {
            web = 3000;
          };
          proxy.port = "web";
        };
      };

    terranix =
      { config, ... }:
      let
        ip = config.infra.vlans.apps.staticAddresses.code-server;
      in
      {
        infra.clients.container-code-server = {
          inherit
            name
            mac
            ip
            ;
          ports = {
            web = 3000;
          };
          proxy.port = "web";
        };

        resource.unifi_client.container_code_server = {
          inherit mac name;
          fixed_ip = ip;
          allow_existing = true;
          blocked = false;
          site = "default";
          skip_forget_on_destroy = false;
        };

        import = [
          {
            to = "unifi_client.container_code_server";
            id = mac;
          }
        ];
      };
  };
}
