# networking — network configuration and hostname settings for desktop host
{ den, inputs, ... }:
{
  den.aspects.hosts._.desktop._.networking = {
    aspect-name = "hosts.desktop.networking";
    includes = [
      den.aspects.services._.resolved
      den.aspects.services._.dns-cache
      den.aspects.networking._.networkmanager
      den.aspects.infra._.networks._.trust
      den.aspects.infra._.clients._.desktop
      den.aspects.infra._.options
      den.aspects.infra._.clients._.container-dns
    ];

    nixos =
      { config, ... }:
      {
        programs.ssh.knownHosts.server = {
          hostNames = [ "server.mgmt.internal" ];
          publicKeyFile = inputs.self + "/pubkeys/server_host_ed25519.pub";
        };

        networking = {
          hostName = "desktop";

          networkmanager.unmanaged = [
            "interface-name:enp9s0"
          ];

          interfaces.enp9s0 = {
            macAddress = config.infra.clients.desktop.mac;
            ipv4.addresses = [
              {
                address = config.infra.clients.desktop.ip;
                prefixLength = config.infra.vlans.trust.subnetPrefix;
              }
            ];
          };

          defaultGateway = config.infra.vlans.trust.staticAddresses.gateway;
          # Single server on purpose: the local split-forwarder (services.dns-cache)
          # decides internal-vs-public per domain, so systemd-resolved has nothing to
          # sticky-fail over to.
          nameservers = [ "127.0.0.55" ];
        };
      };
  };
}
