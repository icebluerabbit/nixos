/*
  _mk-vpn.nix — constructor for `infra.vpns.*` aspects (a WireGuard VPN server).

  Underscore-prefixed so import-tree skips it: this is a function, not a module.

  Clients resolve through the homelab's own DNS container, so the VPN sees the
  same internal names as a LAN client does.

  `wireguard` is held under `ignore_changes`: the controller generates the
  keypair, so reading it back as drift would replace the server and invalidate
  every enrolled peer.
*/
_:
{
  id,
  displayName,
  unifiId,
  subnet,
  port ? 51820,
  wanInterface ? "wan",
  resourceName,
}:
{
  mkOs = {
    infra.vpns.${id} = {
      name = displayName;
      inherit
        subnet
        port
        wanInterface
        unifiId
        ;
    };
  };

  mkTerranix = dnsServers: {
    resource.unifi_vpn_server.${resourceName} = {
      name = displayName;
      enabled = true;
      inherit subnet;
      wan = {
        interface = wanInterface;
        ip = "any";
      };
      wireguard = {
        inherit port;
      };
      site = "default";
      dns = {
        enabled = true;
        servers = dnsServers;
      };

      lifecycle = {
        ignore_changes = [ "wireguard" ];
      };
    };

    import = [
      {
        to = "unifi_vpn_server.${resourceName}";
        id = unifiId;
      }
    ];
  };
}
