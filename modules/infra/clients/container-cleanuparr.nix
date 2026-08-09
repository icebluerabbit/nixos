# modules/infra/clients/container-cleanuparr.nix
{ den, ... }:
let
  name = "Cleanuparr Container";
  # Not vlanId + index: media's contiguous run is exhausted through :8a, and the
  # last byte is a network-wide space, so this is the lowest globally-free slot.
  mac = "62:f5:a4:5b:20:8b";
in
{
  den.aspects.infra._.clients._.container-cleanuparr = {
    aspect-name = "infra.clients.container-cleanuparr";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
    ];

    os =
      { config, ... }:
      {
        infra.clients.container-cleanuparr = {
          inherit name mac;
          ip = config.infra.vlans.media.staticAddresses.cleanuparr;
          ports.default = 11011;
          proxy.port = "default";
        };
      };

    terranix =
      { config, ... }:
      let
        ip = config.infra.vlans.media.staticAddresses.cleanuparr;
      in
      {
        infra.clients.container-cleanuparr = {
          inherit name mac ip;
          ports.default = 11011;
          proxy.port = "default";
        };

        resource.unifi_client.container_cleanuparr = {
          inherit mac name;
          fixed_ip = ip;
          allow_existing = true;
          blocked = false;
          site = "default";
          skip_forget_on_destroy = false;
        };

        # Uncomment after the first successful `just infra-apply-homelab`.
        # import = [
        #   {
        #     to = "unifi_client.container_cleanuparr";
        #     id = mac;
        #   }
        # ];
      };
  };
}
