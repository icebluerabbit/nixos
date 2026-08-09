# home network
{ den, ... }:
let
  name = "seasonpackarr Container";
  mac = "62:f5:a4:5b:20:9a";
in
{
  den.aspects.infra._.clients._.container-seasonpackarr = {
    aspect-name = "infra.clients.container-seasonpackarr";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
    ];

    os =
      { config, ... }:
      let
        ip = config.infra.vlans.media.staticAddresses.seasonpackarr;
      in
      {
        infra.clients.container-seasonpackarr = {
          inherit
            name
            mac
            ip
            ;
          # No proxy.port: seasonpackarr serves only /api/pack, /api/parse and
          # /api/healthz — there is no web UI to reverse-proxy.
          ports = {
            default = 42069;
          };
        };
      };

    terranix =
      { config, ... }:
      let
        ip = config.infra.vlans.media.staticAddresses.seasonpackarr;
      in
      {
        infra.clients.container-seasonpackarr = {
          inherit
            name
            mac
            ip
            ;
          ports = {
            default = 42069;
          };
        };

        resource.unifi_client.container_seasonpackarr = {
          inherit mac name;
          fixed_ip = ip;
          allow_existing = true;
          blocked = false;
          site = "default";
          skip_forget_on_destroy = false;
        };

        # Uncomment after the first `just infra-apply-homelab`.
        # import = [
        #   {
        #     to = "unifi_client.container_seasonpackarr";
        #     id = mac;
        #   }
        # ];
      };
  };
}
