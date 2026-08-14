# modules/infra/clients/container-unpackerr.nix
{ den, lib, ... }:
(import ../_mk-client.nix { inherit den lib; }) {
  id = "container-unpackerr";
  resourceName = "container_unpackerr";
  name = "Unpackerr Container";
  mac = "62:f5:a4:5b:20:81";
  vlan = "media";
  addressKey = "unpackerr";
  # Only listener is the built-in Prometheus metrics webserver.
  ports.metrics = 5656;
}
