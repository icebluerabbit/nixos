# flake-path — where this host's working checkout of this repo lives
#
# Overrides the install-time default (`/src/nixos`, see modules/defaults.nix).
# That copy is made once by the ISO installer and never syncs, so anything
# reading FLAKE against it sees the config as it was on install day.
#
# Two consumers, both silent when wrong:
#   - `nh os switch` with no flakeref builds from here
#   - sys-mcp evaluates `infra` from here (modules/cli/mcp.nix reads FLAKE)
#
# The Justfile is unaffected — its recipes use `.#desktop`, relative to cwd.
_: {
  den.aspects.hosts._.desktop._.flake-path = {
    aspect-name = "hosts.desktop.flake-path";
    nixos = _: {
      environment.variables = {
        FLAKE = "/projects/nixos";
        NH_FLAKE = "/projects/nixos";
      };
    };
  };
}
