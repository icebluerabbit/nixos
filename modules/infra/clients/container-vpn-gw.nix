{ den, ... }:
let
  name = "VPN Gateway Container";
  mac = "62:f5:a4:5b:20:84";
in
{
  den.aspects.infra._.clients._.container-vpn-gw = {
    aspect-name = "infra.clients.container-vpn-gw";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
    ];

    os =
      { config, ... }:
      {
        infra.clients.container-vpn-gw = {
          inherit name mac;
          ip = config.infra.vlans.download.staticAddresses.vpnGw;
          ports = {
            nodeExporter = 9100;
            wireguardExporter = 9586;
            control = 9099;
          };
        };
      };

    terranix =
      { config, ... }:
      {
        infra.clients.container-vpn-gw = {
          inherit name mac;
          ip = config.infra.vlans.download.staticAddresses.vpnGw;
          ports = {
            nodeExporter = 9100;
            wireguardExporter = 9586;
            control = 9099;
          };
        };

        resource.unifi_client.container_vpn_gw = {
          inherit mac name;
          fixed_ip = config.infra.vlans.download.staticAddresses.vpnGw;
          allow_existing = true;
          blocked = false;
          site = "default";
          skip_forget_on_destroy = false;
        };

        import = [
          {
            to = "unifi_client.container_vpn_gw";
            id = mac;
          }
        ];
      };
  };
}
