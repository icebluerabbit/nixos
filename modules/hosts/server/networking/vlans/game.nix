# game — VLAN and bridge interface for game subnet
{ den, ... }:
{
  den.aspects.hosts._.server._.networking._.vlans._.game = {
    aspect-name = "hosts.server.networking.vlans.game";
    includes = [
      den.aspects.hosts._.server._.networking._.bonds
      den.aspects.infra._.networks._.game
      den.aspects.infra._.clients._.server-vlan-game
      den.aspects.infra._.clients._.server-bridge-game
    ];

    nixos =
      { config, ... }:
      {
        networking = {
          vlans.vlan-game = {
            id = config.infra.vlans.game.vlanId;
            interface = "bond0";
          };
          bridges.br-game.interfaces = [ "vlan-game" ];
          interfaces = {
            vlan-game.macAddress = config.infra.clients.server-vlan-game.mac;
            br-game = {
              macAddress = config.infra.clients.server-bridge-game.mac;
              ipv4.addresses = [
                {
                  address = config.infra.clients.server-bridge-game.ip;
                  prefixLength = config.infra.vlans.game.subnetPrefix;
                }
              ];
            };
          };
        };
      };
  };
}
