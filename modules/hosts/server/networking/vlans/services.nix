# services — VLAN and bridge interface for services subnet
{ den, ... }:
{
  den.aspects.hosts._.server._.networking._.vlans._.services = {
    aspect-name = "hosts.server.networking.vlans.services";
    includes = [
      den.aspects.hosts._.server._.networking._.bonds
      den.aspects.infra._.networks._.services
      den.aspects.infra._.clients._.server-vlan-services
      den.aspects.infra._.clients._.server-bridge-services
    ];

    nixos =
      { config, ... }:
      {
        networking = {
          vlans.vlan-services = {
            id = config.infra.vlans.services.vlanId;
            interface = "bond0";
          };
          bridges.br-services.interfaces = [ "vlan-services" ];
          interfaces = {
            vlan-services.macAddress = config.infra.clients.server-vlan-services.mac;
            br-services = {
              macAddress = config.infra.clients.server-bridge-services.mac;
              ipv4.addresses = [
                {
                  address = config.infra.clients.server-bridge-services.ip;
                  prefixLength = config.infra.vlans.services.subnetPrefix;
                }
              ];
            };
          };
        };
      };
  };
}
