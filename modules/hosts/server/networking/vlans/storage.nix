# storage — VLAN and bridge interface for storage subnet
{ den, ... }:
{
  den.aspects.hosts._.server._.networking._.vlans._.storage = {
    aspect-name = "hosts.server.networking.vlans.storage";
    includes = [
      den.aspects.hosts._.server._.networking._.bonds
      den.aspects.infra._.networks._.storage
      den.aspects.infra._.clients._.server-vlan-storage
      den.aspects.infra._.clients._.server-bridge-storage
    ];

    nixos =
      { config, ... }:
      {
        networking = {
          vlans.vlan-storage = {
            id = config.infra.vlans.storage.vlanId;
            interface = "bond0";
          };
          bridges.br-storage.interfaces = [ "vlan-storage" ];
          interfaces = {
            vlan-storage.macAddress = config.infra.clients.server-vlan-storage.mac;
            br-storage = {
              macAddress = config.infra.clients.server-bridge-storage.mac;
              ipv4.addresses = [
                {
                  address = config.infra.clients.server-bridge-storage.ip;
                  prefixLength = config.infra.vlans.storage.subnetPrefix;
                }
              ];
            };
          };
        };
      };
  };
}
