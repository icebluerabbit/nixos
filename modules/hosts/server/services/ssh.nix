# ssh — host configuration override for the SSH daemon bindings
{ den, ... }:
{
  den.aspects.hosts._.server._.services._.ssh = {
    aspect-name = "hosts.server.services.ssh";
    includes = [
      den.aspects.services._.ssh
      den.aspects.infra._.clients._.server
      den.aspects.infra._.clients._.server-bridge-trust
    ];

    nixos =
      { config, ... }:
      {
        services.openssh.listenAddresses = [
          {
            addr = config.infra.clients.server.ip;
            port = 22;
          }
          {
            addr = config.infra.clients.server-bridge-trust.ip;
            port = 22;
          }
        ];
      };
  };
}
