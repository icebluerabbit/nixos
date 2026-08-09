# modules/infra/clients/container-pi-hole.nix
{ den, ... }:
let
  name = "Pi-hole Container";
  mac = "62:f5:a4:5b:20:90";
in
{
  den.aspects.infra._.clients._.container-pi-hole = {
    aspect-name = "infra.clients.container-pi-hole";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
    ];

    os =
      { config, ... }:
      let
        ip = config.infra.vlans.services.staticAddresses.pi-hole;
      in
      {
        infra.clients.container-pi-hole = {
          inherit
            name
            mac
            ip
            ;
          ports = {
            dns = 53;
            web = 80;
          };
          proxy.port = "web";
        };
      };

    terranix =
      { config, ... }:
      let
        ip = config.infra.vlans.services.staticAddresses.pi-hole;
      in
      {
        infra.clients.container-pi-hole = {
          inherit
            name
            mac
            ip
            ;
          ports = {
            dns = 53;
            web = 80;
          };
          proxy.port = "web";
        };

        resource.unifi_client.container_pi_hole = {
          inherit mac name;
          fixed_ip = ip;
          allow_existing = true;
          blocked = false;
          site = "default";
          skip_forget_on_destroy = false;
        };

        import = [
          {
            to = "unifi_client.container_pi_hole";
            id = mac;
          }
        ];
      };
  };
}
