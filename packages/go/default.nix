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
{
  lib,
  buildGoModule,
  makeWrapper,
  chromaprint,
  flac,
}:
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
      # Every binary the module installs. One project ships one binary, so the
      # default is just `program`; a project that has grown a cmd/ layer lists
      # each of them, and the PATH wrapper then reaches all of them.
      programs ? lib.optional (program != null) program,
      # Tools the binaries exec at runtime. A non-empty list wraps each
      # installed binary with them on PATH — the same contract
      # writeShellApplication's runtimeInputs gave the scripts that some of
      # these projects replace, so nothing is assumed to be on the caller's
      # PATH.
      runtimeInputs ? [ ],
    }:
    buildGoModule (
      {
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
      }
      // lib.optionalAttrs (runtimeInputs != [ ]) {
        nativeBuildInputs = [ makeWrapper ];
        postInstall = lib.concatMapStringsSep "\n" (p: ''
          wrapProgram $out/bin/${p} \
            --prefix PATH : ${lib.makeBinPath runtimeInputs}
        '') programs;
      }
    );
in
{
  managarr-crossseed = mkProject {
    module = "managarr-crossseed";
    description = "Serve cross-seed's decision log to managarr";
  };

  managarr-lidarr = mkProject {
    module = "managarr-lidarr";
    description = "Serve Lidarr's history and notifications to managarr";
  };

  managarr-prowlarr = mkProject {
    module = "managarr-prowlarr";
    description = "Serve Prowlarr's search history to managarr";
  };

  managarr-radarr = mkProject {
    module = "managarr-radarr";
    description = "Serve Radarr's history and notifications to managarr";
  };

  managarr-sonarr = mkProject {
    module = "managarr-sonarr";
    description = "Serve Sonarr's history and notifications to managarr";
  };

  managarr-autobrr = mkProject {
    module = "managarr-autobrr";
    description = "Serve autobrr's announce history to managarr";
  };

  managarr-bazarr = mkProject {
    module = "managarr-bazarr";
    description = "Serve Bazarr's subtitle history to managarr";
  };

  managarr-beets = mkProject {
    module = "managarr-beets";
    description = "Serve beets' library to managarr";
  };

  managarr-qbittorrent = mkProject {
    module = "managarr-qbittorrent";
    description = "Serve qBittorrent's state and completions to managarr";
  };

  managarr-seasonpackarr = mkProject {
    module = "managarr-seasonpackarr";
    description = "Relay seasonpackarr's webhook to managarr";
  };

  managarr-slskd = mkProject {
    module = "managarr-slskd";
    description = "Serve slskd's event log to managarr";
  };

  managarr-soularr = mkProject {
    module = "managarr-soularr";
    description = "Relay soularr's events to managarr";
  };

  managarr-unpackerr = mkProject {
    module = "managarr-unpackerr";
    description = "Relay unpackerr's webhook to managarr";
  };

  managarr = mkProject {
    module = "managarr";
    description = "Record events from the download and library stack";
  };

  beets-lidarr-import = mkProject {
    module = "beets-lidarr-import";
    description = "Hand one finished album from beets to Lidarr";
  };

  flac-util = mkProject {
    module = "flac-util";
    description = "Measure FLAC files and record what was found";
    # The reference decoder and the acoustic fingerprinter are consulted as
    # external processes: --flac and --fpcalc name them, and both default to
    # being found on PATH.
    runtimeInputs = [
      flac
      chromaprint
    ];
  };

  forgejo-token-inject = mkProject {
    module = "forgejo-token-inject";
    description = "Splice the forgejo API token into code-server's settings";
  };

  # The first project with a cmd/ layer: three binaries over one shared
  # internal package, so the attribute keeps its name and every
  # ${pkgs.homelab.ha-seeders}/bin/ha-seed-* callsite is unchanged. program
  # is null because no binary is named after the project.
  ha-seeders = mkProject {
    module = "ha-seeders";
    program = null;
    description = "Seed Home Assistant's area, person and config-entry registries";
  };

  hermes-resolve-rooms = mkProject {
    module = "hermes-resolve-rooms";
    description = "Resolve Matrix room aliases to room IDs for Hermes";
  };

  jellyfin-refresh = mkProject {
    module = "jellyfin-refresh";
    description = "Refresh one Jellyfin library path from an *arr hook";
  };

  nixos-notify = mkProject {
    module = "nixos-notify";
    description = "Report a host lifecycle event to MQTT";
  };

  # Three binaries over one shared WebAPI surface, so the attribute keeps its
  # name and every ${pkgs.homelab.qbt-scripts}/bin/qbt-* callsite is
  # unchanged. program is null because no binary is named after the project.
  qbt-scripts = mkProject {
    module = "qbt-scripts";
    program = null;
    description = "qBittorrent watch-dir import, tracker re-announce and peer metrics";
  };

  qui-provision = mkProject {
    module = "qui-provision";
    description = "Register the qBittorrent instance in qui";
  };

  slskd-prune = mkProject {
    module = "slskd-prune";
    description = "Delete slskd searches older than a retention window";
  };

  mousetrap = mkProject {
    module = "mousetrap";
    description = "MyAnonaMouse grab authority";
  };

  soularr-reclaim = mkProject {
    module = "soularr-reclaim";
    description = "Hand stranded Soulseek album folders back to Lidarr";
  };

  sys-mcp = mkProject {
    module = "sys-mcp";
    description = "Resolve and diagnose this homelab over MCP";
  };

  thelounge-seed = mkProject {
    module = "thelounge-seed";
    description = "Merge declared IRC networks into thelounge's user files";
  };

  topology = mkProject {
    module = "topology";
    description = "Render network diagrams from the infra registries";
  };

  tuwunel-init = mkProject {
    module = "tuwunel-init";
    description = "Provision tuwunel users, rooms and spaces";
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
