# hermes — Nous Research Hermes agent gateway
# https://github.com/NousResearch/hermes-agent
{ inputs, ... }:
{
  flake-file.inputs = {
    hermes-flake = {
      # Pinned: upstream HEAD is broken — hermes-tui esbuild "Could not resolve
      # @hermes/shared/charge-settlement" (missing shared module, rev 299e409,
      # 2026-07-19). 29b8cac (2026-07-13) is the last known-good rev. Drop the
      # pin once upstream restores the shared module.
      url = "github:NousResearch/hermes-agent/29b8cacfab18be3ecaaef7c6d0058fd34a01a169";
      inputs.nixpkgs.follows = "nixpkgs";
    };
  };

  den.aspects.containers._.hermes = {
    aspect-name = "containers.hermes";
    includes = [ ];

    provides.to-hosts.nixos =
      { config, lib, ... }:
      let
        hostConfig = config;
      in
      {
        options.containerConfigs.hermes =
          (import ../_options.nix {
            inherit lib;
            defaultPort = 8642;
            defaultUid = 5032;
            defaultGid = 5032;
          })
          // {
            envFile = lib.mkOption {
              type = lib.types.nullOr lib.types.path;
              default = null;
              description = "Host path to the environment file containing secrets to be mounted and loaded";
            };
            provider = lib.mkOption {
              type = lib.types.str;
              default = "anthropic";
              description = "Model provider backend the agent authenticates against (e.g. anthropic, openrouter).";
            };
            model = lib.mkOption {
              type = lib.types.str;
              default = "anthropic/claude-opus-4-8";
              description = "Default model the agent runs on.";
            };
            matrixHomeserver = lib.mkOption {
              type = lib.types.str;
              description = "Base URL of the Matrix homeserver the gateway logs in to.";
            };
            matrixServerName = lib.mkOption {
              type = lib.types.str;
              default = "matrix.services.internal";
              description = "Matrix server_name, used to build the MXID and room aliases.";
            };
            matrixUser = lib.mkOption {
              type = lib.types.str;
              default = "hermes";
              description = "Localpart of the Matrix account the gateway logs in as.";
            };
            matrixAllowedUsers = lib.mkOption {
              type = lib.types.listOf lib.types.str;
              default = [ ];
              description = "MXIDs allowed to talk to the agent. Empty allows nobody.";
            };
            homeRoomAlias = lib.mkOption {
              type = lib.types.nullOr lib.types.str;
              default = null;
              description = ''
                Alias localpart of the room the agent delivers cron and notification
                output to. Matrix mints room IDs server-side at createRoom, so there is
                no ID to declare here — it is resolved from this alias on every start.
                Unset means unattended output has nowhere to go and is dropped.
              '';
            };
            freeResponseRoomAliases = lib.mkOption {
              type = lib.types.listOf lib.types.str;
              default = [ ];
              description = ''
                Alias localparts of rooms exempt from the @mention requirement, which
                is on by default: every message in these rooms is treated as addressed
                to the agent. Resolved to room IDs on start, same as homeRoomAlias.
              '';
            };
            settings = lib.mkOption {
              type = lib.types.attrs;
              default = { };
              description = "Freeform attrs merged into the generated config.yaml";
            };
          };

        config = {
          systemd.services."container@hermes" = {
            serviceConfig.MemoryHigh = lib.mkIf (
              config.containerConfigs.hermes.memoryHigh != null
            ) config.containerConfigs.hermes.memoryHigh;
          };

          containers.hermes =
            let
              cfg = hostConfig.containerConfigs.hermes;
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
                  "/run/secrets/hermes-env" = {
                    hostPath = cfg.envFile;
                    isReadOnly = true;
                  };
                });

              config =
                { pkgs, lib, ... }:
                let
                  stateDir = "/var/lib/hermes";
                  envFile = "${stateDir}/.hermes/.env";
                  roomAlias = alias: "#${alias}:${cfg.matrixServerName}";

                  resolveRoomsScript = pkgs.writeTextFile {
                    name = "hermes-resolve-rooms.py";
                    text = builtins.readFile ./resolve_rooms.py;
                  };
                in
                {
                  imports = [
                    (import ../_base.nix { inherit lib cfg; })
                    inputs.hermes-flake.nixosModules.default
                  ];

                  users.users.hermes = {
                    uid = lib.mkForce cfg.uid;
                    group = "hermes";
                    isSystemUser = true;
                    home = stateDir;
                    createHome = false;
                  };
                  users.groups.hermes.gid = lib.mkForce cfg.gid;

                  services.hermes-agent = {
                    enable = true;
                    user = "hermes";
                    group = "hermes";
                    createUser = false;
                    inherit stateDir;

                    # The matrix and anthropic extras are deliberately excluded from
                    # the package's `all` group (opt-in backends), so pull them in.
                    extraDependencyGroups = [
                      "matrix"
                      "anthropic"
                    ];

                    environmentFiles = lib.optional (cfg.envFile != null) "/run/secrets/hermes-env";

                    environment = {
                      MATRIX_HOMESERVER = cfg.matrixHomeserver;
                      MATRIX_USER_ID = "@${cfg.matrixUser}:${cfg.matrixServerName}";
                    }
                    // lib.optionalAttrs (cfg.matrixAllowedUsers != [ ]) {
                      MATRIX_ALLOWED_USERS = lib.concatStringsSep "," cfg.matrixAllowedUsers;
                    };

                    settings = {
                      model = {
                        inherit (cfg) provider;
                        default = cfg.model;
                      };
                      platforms = {
                        matrix.enabled = true;
                        api_server = {
                          enabled = true;
                          extra = {
                            host = "0.0.0.0";
                            inherit (cfg) port;
                          };
                        };
                      };
                    }
                    // cfg.settings;
                  };

                  # Room IDs cannot be declared: the homeserver mints them at
                  # createRoom and only hands them back over the API. Resolve the
                  # declared aliases and append the IDs to the .env the activation
                  # script has just written. Runs on every start, so a recreated room
                  # heals itself.
                  systemd.services.hermes-resolve-rooms =
                    lib.mkIf (cfg.homeRoomAlias != null || cfg.freeResponseRoomAliases != [ ])
                      {
                        description = "Resolve Hermes Matrix room aliases to room IDs";
                        before = [ "hermes-agent.service" ];
                        requiredBy = [ "hermes-agent.service" ];
                        after = [ "network-online.target" ];
                        wants = [ "network-online.target" ];
                        serviceConfig = {
                          Type = "oneshot";
                          User = "hermes";
                          Group = "hermes";
                          ExecStart = lib.concatStringsSep " " (
                            [
                              (lib.getExe pkgs.python3)
                              resolveRoomsScript
                              cfg.matrixHomeserver
                              envFile
                              (if cfg.homeRoomAlias != null then roomAlias cfg.homeRoomAlias else "-")
                            ]
                            ++ (map roomAlias cfg.freeResponseRoomAliases)
                          );
                        };
                      };
                };
            };
        };
      };
  };
}
