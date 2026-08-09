# work — VLAN and bridge interface for work subnet
{ den, ... }:
{
  den.aspects.hosts._.server._.networking._.vlans._.work = {
    aspect-name = "hosts.server.networking.vlans.work";
    includes = [
      den.aspects.hosts._.server._.networking._.bonds
      den.aspects.infra._.networks._.work
      den.aspects.infra._.clients._.server-vlan-work
      den.aspects.infra._.clients._.server-bridge-work
    ];

    nixos =
      { config, ... }:
      {
        networking = {
          vlans.vlan-work = {
            id = config.infra.vlans.work.vlanId;
            interface = "bond0";
          };
          bridges.br-work.interfaces = [ "vlan-work" ];
          interfaces = {
            vlan-work.macAddress = config.infra.clients.server-vlan-work.mac;
            br-work = {
              macAddress = config.infra.clients.server-bridge-work.mac;
              ipv4.addresses = [
                {
                  address = config.infra.clients.server-bridge-work.ip;
                  prefixLength = config.infra.vlans.work.subnetPrefix;
                }
              ];
            };
          };
        };
      };
  };
}
