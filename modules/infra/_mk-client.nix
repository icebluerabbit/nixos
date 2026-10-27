/*
  _mk-client.nix — builds the `os` and `terranix` bodies for an
  `infra.clients.*` aspect.

  Underscore-prefixed so import-tree skips it: this is a function, not a module.

  Plain data in, plain data out. It never reads `config` and never touches an
  aspect — the aspect file owns its identity, its `includes`, and the lookup of
  its own address, and passes the resolved value in:

      os = { config, ... }: client.mkOs config.infra.vlans.media.staticAddresses.jellyfin;
      terranix = { config, ... }: client.mkTerranix config.infra.vlans.media.staticAddresses.jellyfin;

  Pass `null` for a client with no fixed address.
*/
{ lib }:
{
  id,
  resourceName,
  name,
  mac,
  vlan ? null,
  kind ? "physical",
  portProfile ? null,
  ports ? { },
  proxy ? null,
  blocked ? false,
  groups ? [ ],
  dependsOn ? [ ],
  icon ? null,
  description ? null,
}:
let
  clientOf = ip: {
    inherit
      name
      mac
      vlan
      kind
      portProfile
      blocked
      groups
      ports
      proxy
      icon
      description
      ip
      ;
  };
in
{
  mkOs = ip: {
    infra.clients.${id} = clientOf ip;
  };

  mkTerranix = ip: {
    infra.clients.${id} = clientOf ip;

    resource.unifi_client.${resourceName} = {
      inherit mac name;
      allow_existing = true;
      blocked = false;
      site = "default";
      skip_forget_on_destroy = false;
    }
    // lib.optionalAttrs (ip != null) { fixed_ip = ip; }
    // lib.optionalAttrs (dependsOn != [ ]) {
      depends_on = map (dep: "unifi_client.${dep}") dependsOn;
    };

    import = [
      {
        to = "unifi_client.${resourceName}";
        id = mac;
      }
    ];
  };
}
