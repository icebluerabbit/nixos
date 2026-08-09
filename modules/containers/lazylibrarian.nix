# lazylibrarian — book/audiobook automation (readarr replacement) for Usenet/BitTorrent
# https://gitlab.com/LazyLibrarian/LazyLibrarian
{ inputs, ... }:
{
  den.aspects.containers._.lazylibrarian = {
    aspect-name = "containers.lazylibrarian";
    includes = [ ];

    provides.to-hosts.nixos =
      { config, lib, ... }:
      let
        hostConfig = config;
      in
      {
        options.containerConfigs.lazylibrarian =
          (import ./_options.nix {
            inherit lib;
            defaultPort = 5299;
            defaultUid = 5030;
            defaultGid = 5030;
          })
          // {
            envFile = lib.mkOption {
              type = lib.types.nullOr lib.types.path;
              default = null;
              description = "Host path to the environment file containing secrets to be mounted and loaded";
            };
            configuratarr = lib.mkOption {
              default = { };
              description = "Declarative configuratarr sync for this LazyLibrarian instance";
              type = lib.types.submodule {
                options = {
                  enable = lib.mkEnableOption "configuratarr declarative sync";
                  settings = lib.mkOption {
                    type = lib.types.attrs;
                    default = { };
                    description = "configuratarr settings (verbatim); each top-level key is a named instance";
                  };
                  prune = lib.mkOption {
                    type = lib.types.bool;
                    default = false;
                    description = "Delete server-side resources absent from settings";
                  };
                  waitForHealthy = lib.mkOption {
                    type = lib.types.bool;
                    default = true;
                    description = "Poll each app's health endpoint before syncing";
                  };
                  waitTimeout = lib.mkOption {
                    type = lib.types.ints.unsigned;
                    default = 120;
                    description = "Max seconds to wait per service when waitForHealthy";
                  };
                };
              };
            };
          };

        config = {
          systemd.services."container@lazylibrarian" = {
            serviceConfig.MemoryHigh = lib.mkIf (
              config.containerConfigs.lazylibrarian.memoryHigh != null
            ) config.containerConfigs.lazylibrarian.memoryHigh;
          };

          containers.lazylibrarian =
            let
              cfg = hostConfig.containerConfigs.lazylibrarian;
            in
            {
              inherit (cfg) autoStart;
              ephemeral = true;
              timeoutStartSec = "5min";
              privateNetwork = true;
              inherit (cfg) hostBridge;
              localMacAddress = cfg.macAddress;
              inherit (cfg) localAddress;
              bindMounts =
                cfg.bindMounts
                // (lib.optionalAttrs (cfg.envFile != null) {
                  "/run/secrets/lazylibrarian-env" = {
                    hostPath = cfg.envFile;
                    isReadOnly = true;
                  };
                });

              config =
                { pkgs, lib, ... }:
                let
                  # LazyLibrarian isn't in nixpkgs and its two feature deps
                  # (iso639-lang, slskd-api) aren't packaged — add them here.
                  python = pkgs.python3.override {
                    packageOverrides = self: _super: {
                      iso639-lang = self.buildPythonPackage {
                        pname = "iso639-lang";
                        version = "2.6.3";
                        pyproject = true;
                        src = pkgs.fetchPypi {
                          pname = "iso639_lang";
                          version = "2.6.3";
                          hash = "sha256-B43bfNAYLcwENnaRrMgCLd9xWLbLCfCPeYr4I/qGQmU=";
                        };
                        build-system = [ self.setuptools ];
                        pythonImportsCheck = [ "iso639" ];
                      };
                      slskd-api = self.buildPythonPackage {
                        pname = "slskd-api";
                        version = "0.2.4";
                        pyproject = true;
                        src = pkgs.fetchPypi {
                          pname = "slskd_api";
                          version = "0.2.4";
                          hash = "sha256-7jOSBGu5n4L05GLwQgtPEdwT2ai1hpZBu3h9IaLd09E=";
                        };
                        build-system = [
                          self.setuptools
                          self.setuptools-git-versioning
                        ];
                        dependencies = [ self.requests ];
                        pythonImportsCheck = [ "slskd_api" ];
                      };
                    };
                  };

                  pyEnv = python.withPackages (
                    ps: with ps; [
                      apprise
                      apscheduler
                      beautifulsoup4
                      cherrypy
                      cherrypy-cors
                      deluge-client
                      googletrans
                      html5lib
                      httpagentparser
                      httplib2
                      irc
                      iso639-lang
                      lxml
                      mako
                      pillow
                      pyopenssl
                      pyparsing
                      pypdf
                      python-magic
                      rapidfuzz
                      requests
                      slskd-api
                      tzdata
                      urllib3
                      webencodings
                      xmltodict
                    ]
                  );

                  lazylibrarian = pkgs.stdenvNoCC.mkDerivation {
                    pname = "lazylibrarian";
                    version = "0-unstable-2026-07-07";

                    src = pkgs.fetchFromGitLab {
                      owner = "LazyLibrarian";
                      repo = "LazyLibrarian";
                      rev = "40a389ea9f354dc20d8aac2a07e4a7d05b348783";
                      hash = "sha256-dcC2L7o0wIeE0hEUnwkSpI4hOEiPTh4oBpkwIu1A3KA=";
                    };

                    nativeBuildInputs = [ pkgs.makeWrapper ];
                    dontConfigure = true;
                    dontBuild = true;

                    installPhase = ''
                      runHook preInstall
                      mkdir -p $out/share/lazylibrarian
                      cp -r . $out/share/lazylibrarian/
                      makeWrapper ${pyEnv}/bin/python $out/bin/lazylibrarian \
                        --add-flags "$out/share/lazylibrarian/LazyLibrarian.py"
                      runHook postInstall
                    '';
                  };
                in
                {
                  imports = [
                    (import ./_base.nix { inherit lib cfg; })
                    inputs.configuratarr.nixosModules.default
                  ];

                  # Declarative config sync. Module self-provides its package and
                  # orders after lazylibrarian.service; waitForHealthy polls the
                  # API before syncing. Auth = the seeded api key (below).
                  services.configuratarr = lib.mkIf cfg.configuratarr.enable {
                    enable = true;
                    inherit (cfg.configuratarr)
                      settings
                      prune
                      waitForHealthy
                      waitTimeout
                      ;
                  };

                  users.users.lazylibrarian = {
                    uid = lib.mkForce cfg.uid;
                    group = "lazylibrarian";
                    isSystemUser = true;
                    home = "/var/lib/lazylibrarian";
                    createHome = false;
                  };
                  users.groups.lazylibrarian.gid = lib.mkForce cfg.gid;

                  systemd.services.lazylibrarian = {
                    description = "LazyLibrarian book manager";
                    wantedBy = [ "multi-user.target" ];
                    after = [ "network-online.target" ];
                    wants = [ "network-online.target" ];
                    # LazyLibrarian has no config-via-env; force a known API key into
                    # config.ini's [General] so configuratarr can authenticate. Idempotent,
                    # touches only api_enabled + api_key; leaves everything else intact.
                    preStart = lib.mkIf (cfg.envFile != null) ''
                      ${pkgs.python3}/bin/python3 - <<'PY'
                      import configparser, os
                      p = "/var/lib/lazylibrarian/config.ini"
                      c = configparser.ConfigParser()
                      c.read(p)
                      if not c.has_section("General"):
                          c.add_section("General")
                      c.set("General", "api_enabled", "1")
                      c.set("General", "api_key", os.environ["LAZYLIBRARIAN_API_KEY"])
                      with open(p, "w") as f:
                          c.write(f)
                      PY
                    '';
                    # LazyLibrarian self-restarts once on first run (the launcher
                    # re-execs); run directly under systemd it just exits, so let
                    # systemd bring it back. No start-limit so first-run never trips it.
                    startLimitIntervalSec = 0;
                    serviceConfig = {
                      Type = "simple";
                      User = "lazylibrarian";
                      Group = "lazylibrarian";
                      StateDirectory = "lazylibrarian";
                      EnvironmentFile = lib.mkIf (cfg.envFile != null) "/run/secrets/lazylibrarian-env";
                      ExecStart = "${lazylibrarian}/bin/lazylibrarian --datadir /var/lib/lazylibrarian --port ${toString cfg.port} --nolaunch";
                      Restart = "always";
                      RestartSec = "5s";
                    };
                  };

                  # Feed the shared api-key env into the sync oneshot so
                  # ${env.LAZYLIBRARIAN_API_KEY} / ${env.PROWLARR_API_KEY} resolve.
                  systemd.services.configuratarr = lib.mkIf cfg.configuratarr.enable {
                    serviceConfig.EnvironmentFile = lib.mkIf (cfg.envFile != null) "/run/secrets/lazylibrarian-env";
                  };
                };
            };
        };
      };
  };
}
