# apps — VLAN and bridge interface for apps subnet
{ den, ... }:
{
  den.aspects.hosts._.server._.networking._.vlans._.apps = {
    aspect-name = "hosts.server.networking.vlans.apps";
    includes = [
      den.aspects.hosts._.server._.networking._.bonds
      den.aspects.infra._.networks._.apps
      den.aspects.infra._.clients._.server-vlan-apps
      den.aspects.infra._.clients._.server-bridge-apps
    ];

    nixos =
      { config, ... }:
      {
        networking = {
          vlans.vlan-apps = {
            id = config.infra.vlans.apps.vlanId;
            interface = "bond0";
          };
          bridges.br-apps.interfaces = [ "vlan-apps" ];
          interfaces = {
            vlan-apps.macAddress = config.infra.clients.server-vlan-apps.mac;
            br-apps = {
              macAddress = config.infra.clients.server-bridge-apps.mac;
              ipv4.addresses = [
                {
                  address = config.infra.clients.server-bridge-apps.ip;
                  prefixLength = config.infra.vlans.apps.subnetPrefix;
                }
              ];
            };
          };
        };
      };
  };
}
