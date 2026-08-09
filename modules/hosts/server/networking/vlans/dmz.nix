# dmz — VLAN and bridge interface for dmz subnet
{ den, ... }:
{
  den.aspects.hosts._.server._.networking._.vlans._.dmz = {
    aspect-name = "hosts.server.networking.vlans.dmz";
    includes = [
      den.aspects.hosts._.server._.networking._.bonds
      den.aspects.infra._.networks._.dmz
      den.aspects.infra._.clients._.server-vlan-dmz
      den.aspects.infra._.clients._.server-bridge-dmz
    ];

    nixos =
      { config, ... }:
      {
        networking = {
          vlans.vlan-dmz = {
            id = config.infra.vlans.dmz.vlanId;
            interface = "bond0";
          };
          bridges.br-dmz.interfaces = [ "vlan-dmz" ];
          interfaces = {
            vlan-dmz.macAddress = config.infra.clients.server-vlan-dmz.mac;
            br-dmz = {
              macAddress = config.infra.clients.server-bridge-dmz.mac;
              ipv4.addresses = [
                {
                  address = config.infra.clients.server-bridge-dmz.ip;
                  prefixLength = config.infra.vlans.dmz.subnetPrefix;
                }
              ];
            };
          };
        };
      };
  };
}
