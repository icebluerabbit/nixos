# trust — VLAN and bridge interface for trust subnet
{ den, ... }:
{
  den.aspects.hosts._.server._.networking._.vlans._.trust = {
    aspect-name = "hosts.server.networking.vlans.trust";
    includes = [
      den.aspects.hosts._.server._.networking._.bonds
      den.aspects.infra._.networks._.trust
      den.aspects.infra._.clients._.server-vlan-trust
      den.aspects.infra._.clients._.server-bridge-trust
    ];

    nixos =
      { config, ... }:
      {
        networking = {
          vlans.vlan-trust = {
            id = config.infra.vlans.trust.vlanId;
            interface = "bond0";
          };
          bridges.br-trust.interfaces = [ "vlan-trust" ];
          interfaces = {
            vlan-trust.macAddress = config.infra.clients.server-vlan-trust.mac;
            br-trust = {
              macAddress = config.infra.clients.server-bridge-trust.mac;
              ipv4.addresses = [
                {
                  address = config.infra.clients.server-bridge-trust.ip;
                  prefixLength = config.infra.vlans.trust.subnetPrefix;
                }
              ];
            };
          };
        };
      };
  };
}
