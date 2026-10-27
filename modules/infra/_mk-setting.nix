/*
  _mk-setting.nix — constructor for `infra.settings.*` aspects (site settings).

  Underscore-prefixed so import-tree skips it: this is a function, not a module.

  `unifi_setting` is one flat grab-bag per site — IPS categories, connection
  tracking timeouts, DoH, management access — with no shape worth splitting into
  parameters. `settings` is passed through verbatim; this only supplies the
  aspect envelope and the import that binds it to the existing site.
*/
_:
{
  settings,
  site ? "default",
  resourceName,
}:
{
  mkTerranix = {
    resource.unifi_setting.${resourceName} = {
      inherit site;
    }
    // settings;

    import = [
      {
        to = "unifi_setting.${resourceName}";
        id = site;
      }
    ];
  };
}
