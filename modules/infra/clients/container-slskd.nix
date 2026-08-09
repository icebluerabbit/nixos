{ den, ... }:
let
  name = "slskd Container";
  mac = "62:f5:a4:5b:20:88"; # globally-unique last byte (:87 is beets); MACs must be unique network-wide
in
{
  den.aspects.infra._.clients._.container-slskd = {
    aspect-name = "infra.clients.container-slskd";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
    ];

    os =
      { config, ... }:
      {
        infra.clients.container-slskd = {
          inherit name mac;
          ip = config.infra.vlans.download.staticAddresses.slskd;
          # web UI (LAN). Soulseek inbound rides the VPN tunnel DNAT, not a LAN port.
          ports = {
            web = 5030;
          };
          proxy.port = "web";
        };
      };

    terranix =
      { config, ... }:
      {
        infra.clients.container-slskd = {
          inherit name mac;
          ip = config.infra.vlans.download.staticAddresses.slskd;
          ports = {
            web = 5030;
          };
          proxy.port = "web";
        };

        resource.unifi_client.container_slskd = {
          inherit mac name;
          fixed_ip = config.infra.vlans.download.staticAddresses.slskd;
          allow_existing = true;
          blocked = false;
          site = "default";
          skip_forget_on_destroy = false;
        };

        import = [
          {
            to = "unifi_client.container_slskd";
            id = mac;
          }
        ]; # uncomment AFTER first apply
      };
  };
}
