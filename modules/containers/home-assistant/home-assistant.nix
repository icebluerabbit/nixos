/*
  home-assistant — open-source home automation platform
  https://www.home-assistant.io
*/
_: {
  den.aspects.containers._.home-assistant = {
    aspect-name = "containers.home-assistant";
    includes = [ ];

    provides.to-hosts.nixos =
      {
        config,
        lib,
        pkgs,
        ...
      }:
      let
        hostConfig = config;
        # Resolved from host pkgs so the homelab overlay is in scope; the store
        # path pulls into the container closure via the seed unit.
        haSeeders = pkgs.homelab.ha-seeders;
      in
      {
        options.containerConfigs.home-assistant =
          (import ../_options.nix {
            inherit lib;
            defaultPort = 8123;
            defaultUid = 5012;
            defaultGid = 5012;
          })
          // {
            trustedProxies = lib.mkOption {
              type = lib.types.listOf lib.types.str;
              default = [ ];
              description = "List of trusted proxy IPs for Home Assistant HTTP configuration";
            };
            extraComponents = lib.mkOption {
              type = lib.types.listOf lib.types.str;
              default = [
                "analytics"
                "google_translate"
                "met"
                "radio_browser"
                "shopping_list"
              ];
              description = "Extra Home Assistant components to enable";
            };
            extraPackages = lib.mkOption {
              type = lib.types.functionTo (lib.types.listOf lib.types.package);
              default = _ps: [ ];
              description = "Function that returns extra Python packages to inject into Home Assistant";
            };
            customComponents = lib.mkOption {
              type = lib.types.listOf lib.types.package;
              default = [ ];
              description = "Custom component packages to install into Home Assistant";
            };
            extraConfig = lib.mkOption {
              type = lib.types.attrs;
              default = {
                default_config = { };
              };
              description = "Extra Home Assistant declarative configuration attributes";
            };
            dbHost = lib.mkOption {
              type = lib.types.nullOr lib.types.str;
              default = null;
              description = "Central PostgreSQL database host IP";
            };
            dbPort = lib.mkOption {
              type = lib.types.port;
              default = 5432;
              description = "Central PostgreSQL database host port";
            };
            dbUser = lib.mkOption {
              type = lib.types.str;
              default = "hass";
              description = "PostgreSQL user for Home Assistant";
            };
            dbName = lib.mkOption {
              type = lib.types.str;
              default = "hass";
              description = "PostgreSQL database name for Home Assistant";
            };
            configEntries = lib.mkOption {
              type = lib.types.listOf (
                lib.types.submodule {
                  options = {
                    entry_id = lib.mkOption { type = lib.types.str; };
                    domain = lib.mkOption { type = lib.types.str; };
                    title = lib.mkOption { type = lib.types.str; };
                    data = lib.mkOption {
                      type = lib.types.attrs;
                      default = { };
                    };
                    options = lib.mkOption {
                      type = lib.types.attrs;
                      default = { };
                    };
                    source = lib.mkOption {
                      type = lib.types.str;
                      default = "user";
                    };
                    version = lib.mkOption {
                      type = lib.types.int;
                      default = 1;
                    };
                    minor_version = lib.mkOption {
                      type = lib.types.int;
                      default = 1;
                    };
                    pref_disable_new_entities = lib.mkOption {
                      type = lib.types.bool;
                      default = false;
                    };
                    pref_disable_polling = lib.mkOption {
                      type = lib.types.bool;
                      default = false;
                    };
                    unique_id = lib.mkOption {
                      type = lib.types.nullOr lib.types.str;
                      default = null;
                    };
                    disabled_by = lib.mkOption {
                      type = lib.types.nullOr lib.types.str;
                      default = null;
                    };
                    discovery_keys = lib.mkOption {
                      type = lib.types.attrs;
                      default = { };
                    };
                    subentries = lib.mkOption {
                      type = lib.types.listOf lib.types.attrs;
                      default = [ ];
                    };
                  };
                }
              );
              default = [ ];
              description = "Config entries to seed into .storage/core.config_entries if not already present (matched by entry_id)";
            };
            areas = lib.mkOption {
              type = lib.types.listOf (
                lib.types.submodule {
                  options = {
                    area_id = lib.mkOption { type = lib.types.str; };
                    name = lib.mkOption { type = lib.types.str; };
                    icon = lib.mkOption {
                      type = lib.types.nullOr lib.types.str;
                      default = null;
                    };
                  };
                }
              );
              default = [ ];
              description = "Areas to seed into .storage/core.area_registry if not already present (matched by area_id)";
            };
            persons = lib.mkOption {
              type = lib.types.listOf (
                lib.types.submodule {
                  options = {
                    id = lib.mkOption { type = lib.types.str; };
                    device_trackers = lib.mkOption {
                      type = lib.types.listOf lib.types.str;
                      default = [ ];
                    };
                  };
                }
              );
              default = [ ];
              description = "Device trackers to union into existing .storage/person entries (matched by id; never creates persons).";
            };
            seedEnvironmentFile = lib.mkOption {
              type = lib.types.nullOr lib.types.str;
              default = null;
              description = "EnvironmentFile for the seed oneshot; $VARs referenced in configEntries data expand from it at runtime (keeps secrets out of the Nix store).";
            };
          };

        config = {
          systemd.services."container@home-assistant" = {
            serviceConfig.MemoryHigh = lib.mkIf (
              config.containerConfigs.home-assistant.memoryHigh != null
            ) config.containerConfigs.home-assistant.memoryHigh;
            # Must exceed the in-container home-assistant.service stop timeout
            # (210s) or the host kills nspawn mid-shutdown and hass never
            # finishes its final-write stage. containers.<name> exposes only
            # timeoutStartSec upstream, so this rides raw serviceConfig.
            serviceConfig.TimeoutStopSec = "240s";
          };

          containers.home-assistant =
            let
              cfg = hostConfig.containerConfigs.home-assistant;
            in
            {
              inherit (cfg) autoStart;
              ephemeral = true;
              timeoutStartSec = "5min";
              privateNetwork = true;
              inherit (cfg) hostBridge;
              localMacAddress = cfg.macAddress;
              inherit (cfg) localAddress;
              inherit (cfg) bindMounts;
              config =
                { pkgs, lib, ... }:
                let
                  seedEntries = "${haSeeders}/bin/ha-seed-config-entries";
                  seedAreas = "${haSeeders}/bin/ha-seed-areas";
                  seedPersons = "${haSeeders}/bin/ha-seed-persons";
                in
                {
                  imports = [
                    (import ../_base.nix { inherit lib cfg; })
                  ];

                  # zeroconf/mDNS (port 5353) isn't covered by openFirewall,
                  # which only opens the HTTP port; without it, discovery for
                  # homekit_controller and other auto-discovered integrations
                  # never reaches the container.
                  networking.firewall.allowedUDPPorts = [ 5353 ];

                  services.home-assistant = {
                    enable = true;
                    openFirewall = false;
                    extraComponents = cfg.extraComponents ++ [ "isal" ];
                    extraPackages = ps: (cfg.extraPackages ps) ++ (lib.optional (cfg.dbHost != null) ps.psycopg2);
                    inherit (cfg) customComponents;
                    config =
                      lib.recursiveUpdate
                        (
                          {
                            automation = "!include automations.yaml";
                            script = "!include scripts.yaml";
                            scene = "!include scenes.yaml";
                          }
                          // cfg.extraConfig
                        )
                        (
                          {
                            http = {
                              use_x_forwarded_for = true;
                              trusted_proxies = cfg.trustedProxies;
                            };
                          }
                          // lib.optionalAttrs (cfg.dbHost != null) {
                            recorder.db_url = "postgresql://${cfg.dbUser}@${cfg.dbHost}:${toString cfg.dbPort}/${cfg.dbName}";
                          }
                        );
                  };

                  systemd = {
                    services = {
                      home-assistant = {
                        # hass budgets up to 210s to shut down (core.py: 20s
                        # stopping + 100s stop + 60s final write + 30s close). The
                        # 90s systemd default guarantees a SIGKILL whenever an
                        # integration blocks, and it lands during the final-write
                        # stage that rewrites .storage. Matching its own budget lets
                        # it hit its internal stage timeouts, log the offending
                        # tasks, and exit cleanly. Healthy stops still take ~5s.
                        serviceConfig.TimeoutStopSec = "210s";

                        preStart = lib.mkBefore (
                          lib.optionalString (cfg.dbHost != null) ''
                            until ${pkgs.netcat-openbsd}/bin/nc -z ${cfg.dbHost} ${toString cfg.dbPort}; do
                              echo "Waiting for PostgreSQL database..."
                              sleep 1
                            done
                          ''
                        );
                      };

                      # Vector otherwise stops in the same breath as hass, so
                      # everything hass logs while shutting down — including the
                      # "Timed out waiting for integrations to stop" task dump —
                      # is lost. The container is ephemeral, so its journal goes
                      # with it and Loki is the only durable sink. Before= makes
                      # vector start first and therefore stop last.
                      vector.before = [ "home-assistant.service" ];

                      # Seeding runs in its own oneshot so it can carry an
                      # EnvironmentFile (for $VAR expansion in configEntries) without
                      # polluting the home-assistant service environment.
                      home-assistant-seed =
                        lib.mkIf (cfg.configEntries != [ ] || cfg.areas != [ ] || cfg.persons != [ ])
                          {
                            before = [ "home-assistant.service" ];
                            requiredBy = [ "home-assistant.service" ];
                            serviceConfig = {
                              Type = "oneshot";
                              User = "hass";
                              Group = "hass";
                              RemainAfterExit = true;
                            }
                            // lib.optionalAttrs (cfg.seedEnvironmentFile != null) {
                              EnvironmentFile = cfg.seedEnvironmentFile;
                            };
                            script =
                              lib.optionalString (cfg.configEntries != [ ]) ''
                                printf '%s' ${lib.escapeShellArg (builtins.toJSON cfg.configEntries)} | ${seedEntries}
                              ''
                              + lib.optionalString (cfg.areas != [ ]) ''
                                printf '%s' ${lib.escapeShellArg (builtins.toJSON cfg.areas)} | ${seedAreas}
                              ''
                              + lib.optionalString (cfg.persons != [ ]) ''
                                printf '%s' ${lib.escapeShellArg (builtins.toJSON cfg.persons)} | ${seedPersons}
                              '';
                          };
                    };

                    tmpfiles.rules = [
                      "f /var/lib/hass/automations.yaml 0644 hass hass"
                      "f /var/lib/hass/scenes.yaml 0644 hass hass"
                      "f /var/lib/hass/scripts.yaml 0644 hass hass"
                    ];
                  };

                  users.users.hass = {
                    uid = lib.mkForce cfg.uid;
                    isSystemUser = lib.mkForce false;
                    isNormalUser = true;
                  };
                  users.groups.hass.gid = lib.mkForce cfg.gid;
                };
            };
        };
      };
  };
}
