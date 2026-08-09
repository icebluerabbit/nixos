# home network
{ den, ... }:
let
  name = "Mousetrap Container";
  mac = "62:f5:a4:5b:20:8a";
in
{
  den.aspects.infra._.clients._.container-mousetrap = {
    aspect-name = "infra.clients.container-mousetrap";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
    ];

    os =
      { config, ... }:
      let
        ip = config.infra.vlans.media.staticAddresses.mousetrap;
      in
      {
        infra.clients.container-mousetrap = {
          inherit
            name
            mac
            ip
            ;
          ports = {
            default = 8080;
          };
        };
      };

    terranix =
      { config, ... }:
      let
        ip = config.infra.vlans.media.staticAddresses.mousetrap;
      in
      {
        infra.clients.container-mousetrap = {
          inherit
            name
            mac
            ip
            ;
          ports = {
            default = 8080;
          };
        };

        resource.unifi_client.container_mousetrap = {
          inherit mac name;
          fixed_ip = ip;
          allow_existing = true;
          blocked = false;
          site = "default";
          skip_forget_on_destroy = false;
        };

        # Uncomment AFTER the first `just infra-apply-homelab` registers the client:
        # import = [
        #   {
        #     to = "unifi_client.container_mousetrap";
        #     id = mac;
        #   }
        # ];
      };
  };
}
