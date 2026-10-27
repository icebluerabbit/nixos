# modules/infra/networks/services.nix
{ den, lib, ... }:
let
  network = (import ../_mk-network.nix { inherit lib; }) {
    id = "services";
    resourceName = "services";
    displayName = "SERVICES";
    vlanId = 140;
    subnet = "10.10.140.0";
    domain = "services.internal";
    staticAddresses = {
      gateway = "10.10.140.1";
      server = "10.10.140.2";
      postgres = "10.10.140.3";
      pgadmin = "10.10.140.4";
      dns = "10.10.140.5";
      pi-hole = "10.10.140.6";
      opencode = "10.10.140.7";
      loki = "10.10.140.8";
      grafana = "10.10.140.9";
      prometheus = "10.10.140.10";
      unifi-exporter = "10.10.140.11";
      tempo = "10.10.140.12";
      authelia = "10.10.140.13";
      tuwunel = "10.10.140.14";
      lldap = "10.10.140.15";
      hermes = "10.10.140.16";
    };
  };
in
{
  den.aspects.infra._.networks._.services = {
    aspect-name = "infra.networks.services";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
      den.aspects.infra._.clients._.container-dns
    ];

    os =
      { config, ... }:
      network.mkOs [ config.infra.clients.container-dns.ip ];
    terranix =
      { config, ... }:
      network.mkTerranix [ config.infra.clients.container-dns.ip ];
  };
}
