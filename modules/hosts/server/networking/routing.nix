# routing — external interface, gateway, NAT, and management IP settings
{ den, ... }:
{
  den.aspects.hosts._.server._.networking._.routing = {
    aspect-name = "hosts.server.networking.routing";
    includes = [
      den.aspects.services._.resolved
      den.aspects.networking._.nat
      den.aspects.networking._.networkmanager
      den.aspects.infra._.options
      den.aspects.infra._.networks._.mgmt
      den.aspects.infra._.clients._.server
      den.aspects.infra._.clients._.desktop
      den.aspects.infra._.clients._.container-dns
    ];

    nixos =
      { config, ... }:
      {
        networking = {
          hostName = "server";

          nat.externalInterface = "enp9s0";

          defaultGateway = config.infra.vlans.mgmt.staticAddresses.gateway;

          nameservers = [
            config.infra.clients.container-dns.ip
            "1.1.1.1"
          ];

          interfaces.enp9s0 = {
            macAddress = config.infra.clients.server.mac;
            ipv4.addresses = [
              {
                address = config.infra.clients.server.ip;
                prefixLength = config.infra.vlans.mgmt.subnetPrefix;
              }
            ];
          };

          firewall.checkReversePath = "loose";
          firewall.allowedTCPPorts = [ 11987 ];
        };
      };
  };
}
