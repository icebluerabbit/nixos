# GitHub. One config entry holding the token, one subentry per watched repo
# (the integration's own model — repositories moved from entry options to
# subentries in its minor version 2). Token expands from seedEnvironmentFile at
# seed time; the Nix store only sees the literal $GITHUB_TOKEN reference.
#
# minor_version MUST be 2: async_migrate_entry reads entry.options["repositories"]
# when it sees minor_version 1, which we never set, and would KeyError on load.
{ lib, ... }:
let
  gh = import ../_github-repos.nix { inherit lib; };
in
{
  entry_id = "a1dc3e02-318e-43d9-9029-10fa93a6745b";
  domain = "github";
  title = "GitHub";
  minor_version = 2;
  data.access_token = "$GITHUB_TOKEN";
  # subentry_id is derived from the repo name so the list stays hand-editable
  # without tracking UUIDs, and so re-seeding is idempotent. HA defaults these
  # to ULIDs but nothing validates the format on load.
  subentries = map (repo: {
    subentry_id = builtins.hashString "md5" repo;
    subentry_type = "repository";
    title = repo;
    unique_id = repo;
    data.repository = repo;
  }) gh.repos;
}
