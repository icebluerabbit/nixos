/*
  _mk-provider.nix — constructor for `infra.providers.*` aspects.

  Underscore-prefixed so import-tree skips it: this is a function, not a module.

  A provider aspect does two unrelated things, and this only supplies the
  envelope around them:

    - declares the sops secrets its credentials come from, via
      `provides.to-users.homeManager`, so `sops.secrets` land in the user's
      config rather than the host's;
    - contributes the Terraform provider block, version pin and any variables.

  `terranix` is passed through verbatim because every provider needs a different
  shape there — gitea reads a container's address out of `config`, unifi pins a
  static API URL, cloudflare takes everything from the environment.
*/
_:
{
  secrets,
}:
{
  mkProvides.to-users.homeManager = _: {
    sops.secrets = secrets;
  };
}
