# modules/infra/clients/container-gitea.nix
{ den, lib, ... }:
(import ../_mk-client.nix { inherit den lib; }) {
  id = "container-gitea";
  resourceName = "container_gitea";
  name = "Gitea Container";
  mac = "62:f5:a4:5b:20:73";
  vlan = "apps";
  addressKey = "gitea";
  ports = {
    ssh = 2222; # gitea builtin SSH; high port avoids privileged bind under PrivateUsers
    web = 3000;
  };
  proxy.port = "web";
}
