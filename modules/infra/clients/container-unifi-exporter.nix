# modules/infra/clients/container-unifi-exporter.nix
{ den, ... }:
let
  name = "UniFi Exporter Container";
  mac = "62:f5:a4:5b:20:97";
in
{
  den.aspects.infra._.clients._.container-unifi-exporter = {
    aspect-name = "infra.clients.container-unifi-exporter";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
    ];

    os =
      { config, ... }:
      {
        infra.clients.container-unifi-exporter = {
          inherit name mac;
          ip = config.infra.vlans.services.staticAddresses.unifi-exporter;
          ports = {
            default = 9130;
            syslog = 5140;
          };
        };
      };

    terranix =
      { config, ... }:
      {
        infra.clients.container-unifi-exporter = {
          inherit name mac;
          ip = config.infra.vlans.services.staticAddresses.unifi-exporter;
          ports = {
            default = 9130;
            syslog = 5140;
          };
        };

        resource.unifi_client.container_unifi_exporter = {
          inherit mac name;
          fixed_ip = config.infra.vlans.services.staticAddresses.unifi-exporter;
          allow_existing = true;
          blocked = false;
          site = "default";
        };

        import = [
          {
            to = "unifi_client.container_unifi_exporter";
            id = mac;
          }
        ];
      };
  };
}
