# modules/infra/clients/container-gitea.nix
{ den, ... }:
let
  name = "Gitea Container";
  mac = "62:f5:a4:5b:20:73";
in
{
  den.aspects.infra._.clients._.container-gitea = {
    aspect-name = "infra.clients.container-gitea";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
    ];

    os =
      { config, ... }:
      let
        ip = config.infra.vlans.apps.staticAddresses.gitea;
      in
      {
        infra.clients.container-gitea = {
          inherit
            name
            mac
            ip
            ;
          ports = {
            web = 3000;
            ssh = 2222; # gitea builtin SSH; high port avoids privileged bind under PrivateUsers
          };
          proxy.port = "web";
        };
      };

    terranix =
      { config, ... }:
      let
        ip = config.infra.vlans.apps.staticAddresses.gitea;
      in
      {
        infra.clients.container-gitea = {
          inherit
            name
            mac
            ip
            ;
          ports = {
            web = 3000;
            ssh = 2222; # gitea builtin SSH; high port avoids privileged bind under PrivateUsers
          };
          proxy.port = "web";
        };

        resource.unifi_client.container_gitea = {
          inherit mac name;
          fixed_ip = ip;
          allow_existing = true;
          blocked = false;
          site = "default";
          skip_forget_on_destroy = false;
        };

        import = [
          {
            to = "unifi_client.container_gitea";
            id = mac;
          }
        ];
      };
  };
}
