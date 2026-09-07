/*
  go — the Go workspace holding every binary and library this repo authors.

  Unlike the rest of packages/, this directory is not one package: it is one
  `src` containing several Go modules, and this file returns an attribute set
  of derivations rather than a single one. modules/overlays.nix merges that set
  into `pkgs.homelab.*`, so each binary keeps a flat, unprefixed name — a tool
  moved in here stays reachable as `pkgs.homelab.<name>` with no callsite
  change.

  Module boundaries follow dependency clusters, not tools:

    lib/      module homelab          — app, obs, httpx, config; the shared core
    clients/  module homelab/clients  — one package per external service API
    tools/    module homelab/tools    — cmd/<name> per binary, one vendorHash
                                        for all of them

  Tools whose dependencies are heavy or unshared (sys-mcp, mousetrap,
  flac-util) become their own module here as they migrate, so that a bump to
  one of their dependency trees cannot force a rebuild of the others.

  go.work joins the modules for gopls and `go test ./...`, but GOWORK is off
  in the build: nixpkgs vendors each module separately and a workspace would
  override -mod=vendor.
*/
{
  lib,
  buildGoModule,
  runCommand,
}:
let
  version = "0.1.0";

  # mkModule builds one module out of the shared workspace source. modRoot
  # selects the module; src stays the workspace root so relative `replace`
  # directives pointing at ../lib resolve.
  mkModule =
    {
      name,
      module,
      vendorHash,
      description,
    }:
    buildGoModule {
      pname = name;
      inherit version;
      src = ./.;
      modRoot = module;
      inherit vendorHash;

      env.GOWORK = "off";

      # One ldflag stamps every command in the module, which is why the
      # version lives in homelab/buildinfo rather than a main.version per
      # binary. Nix builds carry no VCS metadata, so debug.ReadBuildInfo
      # cannot supply it.
      ldflags = [
        "-s"
        "-w"
        "-X homelab/buildinfo.Version=${version}"
      ];

      meta = {
        inherit description;
        mainProgram = name;
      };
    };

  # The tools module compiles as a unit: one vendor tree, one build, every
  # cmd/<name> binary in a single output.
  tools = mkModule {
    name = "homelab-tools";
    module = "tools";
    vendorHash = "sha256-a6wFhRW255G9oGf4g9LGe6A6m7ro0JH+CFvhEzJzd90=";
    description = "Homelab command-line tools";
  };

  # pick extracts one binary from that shared build so each tool keeps its own
  # store path, closure and `pkgs.homelab.<name>` attribute.
  #
  # cp, not a symlink: a link would drag the whole multi-binary build into
  # every consumer's closure, which for a container means shipping two dozen
  # unrelated tools to run one.
  pick =
    name: description:
    runCommand name
      {
        meta = {
          inherit description;
          mainProgram = name;
        };
      }
      ''
        mkdir -p "$out/bin"
        cp ${tools}/bin/${name} "$out/bin/"
      '';

  # Every binary the tools module produces, and what it is for.
  toolPrograms = {
    cleanuparr-setup = "Complete Cleanuparr first-run setup and export its API key";
  };
in
lib.mapAttrs pick toolPrograms
// {
  # The shared core and the service clients carry no binaries; they are built
  # so that `nix flake check` and every `just build-*` run their tests.
  homelab-go-lib = mkModule {
    name = "homelab-go-lib";
    module = "lib";
    vendorHash = "sha256-Rfb3GREwtnzo/mdkiu6UklcprBB6fUTV1r2LmKwD1Yg=";
    description = "Shared bootstrap, logging, HTTP and configuration core for homelab tools";
  };

  homelab-go-clients = mkModule {
    name = "homelab-go-clients";
    module = "clients";
    vendorHash = "sha256-wc9Nnq4waNF1H9ehRXRJprl1M2/ddF8Ws1gCOuRq+Ww=";
    description = "Service API clients for homelab tools";
  };
}
