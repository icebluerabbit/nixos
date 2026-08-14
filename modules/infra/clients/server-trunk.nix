# home network
{ den, lib, ... }:
(import ../_mk-client.nix { inherit den lib; }) {
  id = "server-trunk";
  resourceName = "server_trunk";
  name = "Server Trunk";
  mac = "62:f5:a4:5b:20:b1";
}
