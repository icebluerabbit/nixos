# home network
{ den, ... }:
{
  den.aspects.infra._.vpns._.one-click-vpn = {
    aspect-name = "infra.vpns.one-click-vpn";
    includes = [ den.aspects.infra._.providers._.unifi ];
    terranix =
      { config, ... }:
      {
        resource.unifi_vpn_server.one_click_vpn = {
          name = "One-Click VPN";
          enabled = true;
          subnet = "192.168.4.1/24";
          wan = {
            interface = "wan";
            ip = "any";
          };
          wireguard = {
            port = 51820;
          };
          site = "default";
          dns = {
            enabled = true;
            servers = [ config.infra.clients.container-dns.ip ];
          };

          lifecycle = {
            ignore_changes = [ "wireguard" ];
          };
        };

        import = [
          {
            to = "unifi_vpn_server.one_click_vpn";
            id = "6a2cdd81efa3052c88b365dc";
          }
        ];
      };
  };
}
