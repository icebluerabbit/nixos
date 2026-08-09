# modules/infra/clients/container-lldap.nix
{ den, ... }:
let
  name = "LLDAP Container";
  mac = "62:f5:a4:5b:20:92";
in
{
  den.aspects.infra._.clients._.container-lldap = {
    aspect-name = "infra.clients.container-lldap";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
    ];

    os =
      { config, ... }:
      let
        ip = config.infra.vlans.services.staticAddresses.lldap;
      in
      {
        infra.clients.container-lldap = {
          inherit
            name
            mac
            ip
            ;
          ports = {
            ldap = 3890;
            http = 17170;
          };
          proxy.port = "http";
        };
      };

    terranix =
      { config, ... }:
      let
        ip = config.infra.vlans.services.staticAddresses.lldap;
      in
      {
        infra.clients.container-lldap = {
          inherit
            name
            mac
            ip
            ;
          ports = {
            ldap = 3890;
            http = 17170;
          };
          proxy.port = "http";
        };

        resource.unifi_client.container_lldap = {
          inherit mac name;
          fixed_ip = ip;
          allow_existing = true;
          blocked = false;
          site = "default";
          skip_forget_on_destroy = false;
        };

        import = [
          {
            to = "unifi_client.container_lldap";
            id = mac;
          }
        ];
      };
  };
}
