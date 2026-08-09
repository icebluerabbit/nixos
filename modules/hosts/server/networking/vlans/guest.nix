# guest — VLAN and bridge interface for guest subnet
{ den, ... }:
{
  den.aspects.hosts._.server._.networking._.vlans._.guest = {
    aspect-name = "hosts.server.networking.vlans.guest";
    includes = [
      den.aspects.hosts._.server._.networking._.bonds
      den.aspects.infra._.networks._.guest
      den.aspects.infra._.clients._.server-vlan-guest
      den.aspects.infra._.clients._.server-bridge-guest
    ];

    nixos =
      { config, ... }:
      {
        networking = {
          vlans.vlan-guest = {
            id = config.infra.vlans.guest.vlanId;
            interface = "bond0";
          };
          bridges.br-guest.interfaces = [ "vlan-guest" ];
          interfaces = {
            vlan-guest.macAddress = config.infra.clients.server-vlan-guest.mac;
            br-guest = {
              macAddress = config.infra.clients.server-bridge-guest.mac;
              ipv4.addresses = [
                {
                  address = config.infra.clients.server-bridge-guest.ip;
                  prefixLength = config.infra.vlans.guest.subnetPrefix;
                }
              ];
            };
          };
        };
      };
  };
}
