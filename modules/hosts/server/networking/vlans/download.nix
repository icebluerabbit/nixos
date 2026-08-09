# download — VLAN and bridge interface for download subnet
{ den, ... }:
{
  den.aspects.hosts._.server._.networking._.vlans._.download = {
    aspect-name = "hosts.server.networking.vlans.download";
    includes = [
      den.aspects.hosts._.server._.networking._.bonds
      den.aspects.infra._.networks._.download
      den.aspects.infra._.clients._.server-vlan-download
      den.aspects.infra._.clients._.server-bridge-download
    ];

    nixos =
      { config, ... }:
      {
        networking = {
          vlans.vlan-download = {
            id = config.infra.vlans.download.vlanId;
            interface = "bond0";
          };
          bridges.br-download.interfaces = [ "vlan-download" ];
          interfaces = {
            vlan-download.macAddress = config.infra.clients.server-vlan-download.mac;
            br-download = {
              macAddress = config.infra.clients.server-bridge-download.mac;
              ipv4.addresses = [
                {
                  address = config.infra.clients.server-bridge-download.ip;
                  prefixLength = config.infra.vlans.download.subnetPrefix;
                }
              ];
            };
          };
        };
      };
  };
}
