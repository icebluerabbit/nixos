# media — VLAN and bridge interface for media subnet
{ den, ... }:
{
  den.aspects.hosts._.server._.networking._.vlans._.media = {
    aspect-name = "hosts.server.networking.vlans.media";
    includes = [
      den.aspects.hosts._.server._.networking._.bonds
      den.aspects.infra._.networks._.media
      den.aspects.infra._.clients._.server-vlan-media
      den.aspects.infra._.clients._.server-bridge-media
    ];

    nixos =
      { config, ... }:
      {
        networking = {
          vlans.vlan-media = {
            id = config.infra.vlans.media.vlanId;
            interface = "bond0";
          };
          bridges.br-media.interfaces = [ "vlan-media" ];
          interfaces = {
            vlan-media.macAddress = config.infra.clients.server-vlan-media.mac;
            br-media = {
              macAddress = config.infra.clients.server-bridge-media.mac;
              ipv4.addresses = [
                {
                  address = config.infra.clients.server-bridge-media.ip;
                  prefixLength = config.infra.vlans.media.subnetPrefix;
                }
              ];
            };
          };
        };
      };
  };
}
