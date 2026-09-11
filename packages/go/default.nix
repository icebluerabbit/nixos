/*
  go — the Go workspace holding every binary and library this repo authors.

  Unlike the rest of packages/, this directory is not one package: it is one
  `src` containing several Go modules, and this file returns an attribute set
  of derivations rather than a single one. modules/overlays.nix merges that set
  into `pkgs.homelab.*`, so each binary keeps a flat, unprefixed name — a tool
  moved in here stays reachable as `pkgs.homelab.<name>` with no callsite
  change.

  One directory per project, one project per module, flat:

    lib/              module homelab      shared code; ships no binaries
    <project>/        module homelab/<project>
      cmd/<name>/     the binary
      internal/       everything else it needs

  There is no grouping layer and no shared-vendor bucket. Each project vendors
  only what it imports, so a dependency bump for one binary cannot rebuild any
  other — which is worth more than the duplicated vendor trees it costs.

  go.work joins the modules for gopls and `go test ./...`, but GOWORK is off in
  the build: nixpkgs vendors each module separately and a workspace would
  override -mod=vendor.
*/
{ lib, buildGoModule }:
let
  version = "0.1.0";
  hashes = import ./vendor-hashes.nix;

  # mkProject builds one module out of the shared workspace source. modRoot
  # selects the module; src keeps the workspace root as its root so the
  # relative `replace` directives pointing at ../lib resolve.
  #
  # The fileset is what keeps projects independent. `src = ./.` would put every
  # file under packages/go into every project's source hash, so editing one
  # binary would rebuild all of them at new store paths — and every container
  # embedding those store paths would restart on the next switch, for a binary
  # whose behaviour did not change. Narrowing src to lib plus the project's own
  # directory is what makes the isolation the vendor split promises actually
  # hold: a lib change still rebuilds everything, which is correct, and a
  # project change rebuilds only itself.
  mkProject =
    {
      module,
      description,
      # The binary this module produces. null for lib, which produces none and
      # is built only so its tests run.
      program ? module,
      pname ? module,
    }:
    buildGoModule {
      inherit pname version;
      src = lib.fileset.toSource {
        root = ./.;
        fileset = lib.fileset.unions ([ ./lib ] ++ lib.optional (module != "lib") (./. + "/${module}"));
      };
      modRoot = module;
      vendorHash = hashes.${module};

      env.GOWORK = "off";

      # One ldflag stamps every binary in the module, which is why the version
      # lives in homelab/buildinfo rather than a main.version per binary. Nix
      # builds carry no VCS metadata, so debug.ReadBuildInfo cannot supply it.
      ldflags = [
        "-s"
        "-w"
        "-X homelab/buildinfo.Version=${version}"
      ];

      meta = {
        inherit description;
      }
      // (if program == null then { } else { mainProgram = program; });
    };
in
{
  cleanuparr-setup = mkProject {
    module = "cleanuparr-setup";
    description = "Complete Cleanuparr first-run setup and export its API key";
  };

  slskd-prune = mkProject {
    module = "slskd-prune";
    description = "Delete slskd searches older than a retention window";
  };

  stashtrap = mkProject {
    module = "stashtrap";
    description = "Record autobrr announces and decide whether to grab them";
  };

  # lib ships no binaries; it is built so that `nix flake check` and every
  # `just build-*` run its tests. Its attribute is prefixed because `lib` alone
  # would read like nixpkgs' own.
  homelab-go-lib = mkProject {
    module = "lib";
    pname = "homelab-go-lib";
    program = null;
    description = "Shared bootstrap, logging, HTTP, secret and client code for homelab tools";
  };
}
