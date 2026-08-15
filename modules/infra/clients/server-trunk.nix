# home network
{ den, lib, ... }:
let
  client = (import ../_mk-client.nix { inherit lib; }) {
    id = "server-trunk";
    resourceName = "server_trunk";
    name = "Server Trunk";
    mac = "62:f5:a4:5b:20:b1";
    kind = "virtual";
    portProfile = "server-trunk";
  };
in
{
  den.aspects.infra._.clients._.server-trunk = {
    aspect-name = "infra.clients.server-trunk";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
      den.aspects.infra._.ports._.server-trunk
    ];

    os = _: client.mkOs null;
    terranix = _: client.mkTerranix null;
  };
}
