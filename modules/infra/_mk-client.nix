/*
  _mk-client.nix — constructor for `infra.clients.*` aspects.

  Underscore-prefixed so import-tree skips it: this is a function, not a module.

  Every client aspect was previously ~65 lines of copy-pasted boilerplate that
  repeated the same attrset across the `os` and `terranix` engines. This collapses
  that to a single call describing only what varies.

  An address comes from exactly one of:
    - `vlan` + `addressKey` — resolved from that VLAN's `staticAddresses`
    - `ip`                  — a literal, for clients not yet in `staticAddresses`
    - neither               — no fixed IP (trunk/bridge MACs UniFi never assigns)

  Every client gets an import block keyed on its MAC. Import blocks are
  idempotent once the resource is in state, so they stay in the config
  permanently — a client without one is an oversight, not a state.
*/
{ den, lib }:
{
  id,
  name,
  mac,
  vlan ? null,
  addressKey ? null,
  ip ? null,
  ports ? { },
  proxy ? null,
  blocked ? false,
  groups ? [ ],
  resourceName,
  dependsOn ? [ ],
}:
assert lib.assertMsg (
  !(addressKey != null && ip != null)
) "mkClient ${id}: set addressKey or ip, not both";
assert lib.assertMsg (
  (vlan == null) == (addressKey == null)
) "mkClient ${id}: vlan and addressKey go together";
let
  clientAttrs = config: {
    inherit
      name
      mac
      blocked
      groups
      ports
      proxy
      ;
    ip = if addressKey != null then config.infra.vlans.${vlan}.staticAddresses.${addressKey} else ip;
  };
in
{
  den.aspects.infra._.clients._.${id} = {
    aspect-name = "infra.clients.${id}";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
    ];

    os =
      { config, ... }:
      {
        infra.clients.${id} = clientAttrs config;
      };

    terranix =
      { config, ... }:
      let
        client = clientAttrs config;
      in
      {
        infra.clients.${id} = client;

        resource.unifi_client.${resourceName} = {
          inherit mac name;
          allow_existing = true;
          blocked = false;
          site = "default";
          skip_forget_on_destroy = false;
        }
        // lib.optionalAttrs (client.ip != null) { fixed_ip = client.ip; }
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
  };
}
