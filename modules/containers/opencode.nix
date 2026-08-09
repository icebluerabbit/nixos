# opencode — AI coding agent server
# https://github.com/anomalyco/opencode
_: {
  den.aspects.containers._.opencode = {
    aspect-name = "containers.opencode";
    includes = [ ];

    provides.to-hosts.nixos =
      { config, lib, ... }:
      let
        hostConfig = config;
      in
      {
        options.containerConfigs.opencode =
          (import ./_options.nix {
            inherit lib;
            defaultPort = 3000;
            defaultUid = 5015;
            defaultGid = 5015;
          })
          // {
            envFile = lib.mkOption {
              type = lib.types.nullOr lib.types.path;
              default = null;
              description = "Host path to the environment file containing secrets to be mounted and loaded";
            };
            settings = lib.mkOption {
              type = lib.types.attrs;
              default = { };
              description = "Freeform attrs merged into opencode.json";
            };
            otlpEndpoint = lib.mkOption {
              type = lib.types.nullOr lib.types.str;
              default = null;
              description = "OTLP base URL for opencode-plugin-otel (e.g. http://tempo:4318). Enables telemetry when set.";
            };
            workingDirectory = lib.mkOption {
              type = lib.types.path;
              default = "/mnt/projects";
              description = "Project root opencode serves from (the agent's cwd).";
            };
          };

        config = {
          systemd.services."container@opencode" = {
            serviceConfig.MemoryHigh = lib.mkIf (
              config.containerConfigs.opencode.memoryHigh != null
            ) config.containerConfigs.opencode.memoryHigh;
          };

          containers.opencode =
            let
              cfg = hostConfig.containerConfigs.opencode;
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
                  "/run/secrets/opencode-env" = {
                    hostPath = cfg.envFile;
                    isReadOnly = true;
                  };
                });

              config =
                {
                  config,
                  pkgs,
                  lib,
                  ...
                }:
                let
                  opencode-plugin-otel = pkgs.stdenv.mkDerivation {
                    pname = "opencode-plugin-otel";
                    version = "1.2.1";

                    src = pkgs.fetchFromGitHub {
                      owner = "DEVtheOPS";
                      repo = "opencode-plugin-otel";
                      rev = "09b039d0d42034f375b1aa33b4e2c81cc9204de6";
                      hash = "sha256-jZ/h7gmQAIHozSRFucnSUoDQCzSaed1EMtRAe1KxReI=";
                    };

                    nativeBuildInputs = [ pkgs.bun ];

                    impureEnvVars = lib.fetchers.proxyImpureEnvVars;

                    buildPhase = ''
                      runHook preBuild
                      export HOME=$(mktemp -d)
                      export BUN_INSTALL_CACHE_DIR=$(mktemp -d)
                      bun install --frozen-lockfile --ignore-scripts --no-progress
                      bun build src/index.ts --outdir=./dist --target=node
                      runHook postBuild
                    '';

                    installPhase = ''
                      runHook preInstall
                      mkdir -p $out/dist
                      cp dist/index.js $out/dist/
                      runHook postInstall
                    '';

                    outputHashAlgo = "sha256";
                    outputHashMode = "recursive";
                    outputHash = "sha256-UKm8ZE0J0pQqmOrDawOOdcGp714h1K869Xnny89Bneo=";
                  };

                  # opencode runs every agent command as `$shell -c "<cmd>"`, which is
                  # non-interactive — direnv's shell hook never fires. Loading it here
                  # puts each command in its project's devshell with nothing for the
                  # agent to remember. `direnv export` keys off $PWD, prints nothing
                  # when there is no .envrc, and re-runs per command, so an edited
                  # .envrc reloads on its own.
                  #
                  # Named `bash` deliberately: opencode only routes agent tool calls
                  # through `shell` when it recognises it as a known shell, so a
                  # descriptive name gets it used for the terminal and ignored where
                  # it matters.
                  opencodeShell = pkgs.writeShellScriptBin "bash" ''
                    eval "$(${lib.getExe pkgs.direnv} export bash 2>/dev/null)" || true
                    exec ${lib.getExe pkgs.bashInteractive} "$@"
                  '';
                in
                {
                  imports = [ (import ./_base.nix { inherit lib cfg; }) ];

                  users.users.opencode = {
                    uid = lib.mkForce cfg.uid;
                    group = "opencode";
                    isSystemUser = true;
                    home = "/home/opencode";
                    createHome = false;
                    # without this the user gets `pkgs.shadow` (nologin) — the option
                    # default for system users — and every agent shell-out fails
                    shell = pkgs.bashInteractive;
                  };
                  users.groups.opencode.gid = lib.mkForce cfg.gid;

                  # Agents reach for tools with `nix shell nixpkgs#x -c …` and enter
                  # project devshells with `nix develop -c …`; both need the flake CLI.
                  # The host's /nix/store and daemon socket are already bind-mounted, so
                  # builds land in the host store and outlive this ephemeral container.
                  # Pinning the registry/nixPath to pkgs.path (a store path that is
                  # therefore already present) keeps `nixpkgs#…` resolution local,
                  # offline, and on the same nixpkgs this host is built from — without
                  # it a bare `nixpkgs` reference fetches and evaluates from GitHub.
                  nix = {
                    settings.experimental-features = [
                      "nix-command"
                      "flakes"
                    ];
                    registry.nixpkgs.to = {
                      type = "path";
                      inherit (pkgs) path;
                    };
                    nixPath = [ "nixpkgs=${pkgs.path}" ];
                  };

                  # Without a whitelist direnv refuses every .envrc it wasn't told to
                  # trust, and the wrapper above silently becomes a plain bash. The
                  # agent can already run arbitrary commands under this tree, so
                  # trusting .envrc files in it grants nothing new.
                  environment.etc."opencode-xdg/direnv/direnv.toml".text = ''
                    [whitelist]
                    prefix = [ "${cfg.workingDirectory}" ]
                  '';

                  environment.etc."opencode-xdg/opencode/opencode.json" = {
                    text = builtins.toJSON (
                      {
                        "$schema" = "https://opencode.ai/config.json";
                        shell = "${opencodeShell}/bin/bash";
                      }
                      // lib.optionalAttrs (cfg.otlpEndpoint != null) {
                        plugin = [ "${opencode-plugin-otel}/dist/index.js" ];
                      }
                      // cfg.settings
                    );
                  };

                  systemd.services.opencode = {
                    description = "OpenCode AI coding agent";
                    wantedBy = [ "multi-user.target" ];
                    after = [ "network-online.target" ];
                    wants = [ "network-online.target" ];
                    # systemd's default PATH omits the system profile, so without this
                    # nothing in environment.systemPackages reaches the agent's shells —
                    # they inherit the service's PATH, not a login one.
                    path = [ config.system.path ];
                    environment = {
                      HOME = "/home/opencode";
                      XDG_CONFIG_HOME = "/etc/opencode-xdg";
                    }
                    // lib.optionalAttrs (cfg.otlpEndpoint != null) {
                      OPENCODE_ENABLE_TELEMETRY = "1";
                      OPENCODE_OTLP_ENDPOINT = cfg.otlpEndpoint;
                      OPENCODE_OTLP_PROTOCOL = "http/protobuf";
                      OPENCODE_METRIC_PREFIX = "opencode.";
                    };
                    serviceConfig = {
                      Type = "simple";
                      User = "opencode";
                      Group = "opencode";
                      ExecStart = "${pkgs.opencode}/bin/opencode serve --hostname 0.0.0.0 --port ${toString cfg.port}";
                      EnvironmentFile = lib.mkIf (cfg.envFile != null) "/run/secrets/opencode-env";
                      WorkingDirectory = cfg.workingDirectory;
                      Restart = "on-failure";
                      RestartSec = "5s";
                    };
                  };
                };
            };
        };
      };
  };
}
