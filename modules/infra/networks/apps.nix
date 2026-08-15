# modules/infra/networks/apps.nix
{ den, lib, ... }:
let
  network = (import ../_mk-network.nix { inherit lib; }) {
    id = "apps";
    resourceName = "apps";
    displayName = "APPS";
    vlanId = 110;
    subnet = "10.10.110.0";
    domain = "apps.internal";
    staticAddresses = {
      gateway = "10.10.110.1";
      server = "10.10.110.2";
      # 10.10.110.3 — RESERVED / decommissioned (ex-flood, removed 2026-07, replaced
      # by qui at .11). No container backs it. Future LLMs: do NOT assign .3 to a new
      # container — skip it even though next_free_ip will offer it (the slot is free).
      caddy = "10.10.110.4";
      homepage = "10.10.110.5";
      kiwix = "10.10.110.6";
      gitea = "10.10.110.7";
      openchamber = "10.10.110.8";
      cinny = "10.10.110.9";
      thelounge = "10.10.110.10";
      qui = "10.10.110.11";
      code-server = "10.10.110.3";
    };
  };
in
{
  den.aspects.infra._.networks._.apps = {
    aspect-name = "infra.networks.apps";
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
