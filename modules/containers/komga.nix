/*
  komga — comic and manga server (series/volume model, ComicInfo.xml, OPDS)
  https://komga.org
*/
{ inputs, ... }:
{
  den.aspects.containers._.komga = {
    aspect-name = "containers.komga";
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
      in
      {
        options.containerConfigs.komga =
          (import ./_options.nix {
            inherit lib;
            defaultPort = 25600;
            defaultUid = 5039;
            defaultGid = 5039;
          })
          // {
            stateDir = lib.mkOption {
              type = lib.types.str;
              default = "/var/lib/komga";
              description = "Where Komga keeps its database and thumbnails (local, not the NFS library)";
            };
            extraSettings = lib.mkOption {
              type = lib.types.attrs;
              default = { };
              description = "Additional Komga/Spring settings merged into services.komga.settings";
            };
            envFile = lib.mkOption {
              type = lib.types.nullOr lib.types.path;
              default = null;
              description = "Host path to the environment file containing secrets to be mounted and loaded";
            };
            configuratarr = lib.mkOption {
              default = { };
              description = "Declarative configuratarr sync for this Komga instance";
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
          systemd.services."container@komga" = {
            serviceConfig.MemoryHigh = lib.mkIf (
              config.containerConfigs.komga.memoryHigh != null
            ) config.containerConfigs.komga.memoryHigh;
          };

          containers.komga =
            let
              cfg = hostConfig.containerConfigs.komga;
              # Resolved from host pkgs (the homelab overlay is not in the
              # container's nixpkgs). Only the URL and the key path are argv —
              # the admin credentials still arrive via EnvironmentFile.
              setupCmd = lib.escapeShellArgs [
                (lib.getExe pkgs.homelab.komga-setup)
                "--base"
                "http://127.0.0.1:${toString cfg.port}"
                "--key-file"
                "/run/komga/api-key.env"
              ];
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
                  "/run/secrets/komga-env" = {
                    hostPath = cfg.envFile;
                    isReadOnly = true;
                  };
                });

              config =
                {
                  pkgs,
                  lib,
                  ...
                }:
                let
                  # Komga's OIDC back-channel to Authelia runs over the
                  # internal-CA vhost (Authelia validates the Host header and
                  # answers 400 on a bare IP, so plain HTTP is not an option).
                  # A JVM cannot use a PEM bundle as a truststore, so import the
                  # CA into a copy of the JDK's own cacerts and point the JVM at
                  # that. `changeit` is the JDK's stock store password — this
                  # file holds only public certificates, nothing secret.
                  truststore =
                    pkgs.runCommand "komga-truststore"
                      {
                        nativeBuildInputs = [ pkgs.jdk_headless ];
                      }
                      ''
                        cp ${pkgs.jdk_headless}/lib/openjdk/lib/security/cacerts ./cacerts
                        chmod +w ./cacerts
                        keytool -importcert -noprompt -trustcacerts \
                          -alias homelab-ca \
                          -file ${inputs.self + "/certs/ca.crt"} \
                          -keystore ./cacerts -storepass changeit
                        cp ./cacerts $out
                      '';
                in
                {
                  imports = [
                    (import ./_base.nix { inherit lib cfg; })
                    inputs.configuratarr.nixosModules.default
                  ];

                  services.komga = {
                    enable = true;
                    user = "komga";
                    group = "komga";
                    openFirewall = false;
                    inherit (cfg) stateDir;
                    settings = {
                      server.port = cfg.port;
                    }
                    // cfg.extraSettings;
                  };

                  services.configuratarr = lib.mkIf cfg.configuratarr.enable {
                    enable = true;
                    inherit (cfg.configuratarr)
                      settings
                      prune
                      waitForHealthy
                      waitTimeout
                      ;
                  };

                  systemd.services = {
                    komga = {
                      # Komga itself needs the env file too, not just the setup
                      # and configuratarr units: Spring resolves
                      # ${KOMGA_OIDC_CLIENT_SECRET} in application.yml from the
                      # process environment, and without it Authelia rejects the
                      # token exchange with "Client authentication failed".
                      serviceConfig.EnvironmentFile = lib.mkIf (cfg.envFile != null) "/run/secrets/komga-env";
                      environment.JAVA_TOOL_OPTIONS =
                        "-Djavax.net.ssl.trustStore=${truststore} " + "-Djavax.net.ssl.trustStorePassword=changeit";
                    };

                    # Komga has no env var that seeds the initial admin, so claim
                    # the server over its API and mint the key configuratarr
                    # authenticates with. Idempotent across reboots: the account
                    # lives in stateDir, not the ephemeral rootfs.
                    komga-setup = lib.mkIf cfg.configuratarr.enable {
                      description = "Claim Komga and export its API key";
                      after = [ "komga.service" ];
                      requires = [ "komga.service" ];
                      before = [ "configuratarr.service" ];
                      wantedBy = [ "multi-user.target" ];
                      # No `path` — komga-setup pins its own curl and jq.
                      serviceConfig = {
                        Type = "oneshot";
                        RemainAfterExit = true;
                        RuntimeDirectory = "komga";
                        RuntimeDirectoryMode = "0700";
                        # configuratarr.service reads the key file, so it has to
                        # outlive this oneshot.
                        RuntimeDirectoryPreserve = "yes";
                        EnvironmentFile = lib.mkIf (cfg.envFile != null) "/run/secrets/komga-env";
                        ExecStart = setupCmd;
                      };
                    };

                    configuratarr = lib.mkIf cfg.configuratarr.enable {
                      after = [ "komga-setup.service" ];
                      requires = [ "komga-setup.service" ];
                      serviceConfig.EnvironmentFile = [
                        "/run/komga/api-key.env"
                      ]
                      ++ lib.optional (cfg.envFile != null) "/run/secrets/komga-env";
                    };
                  };

                  # Pin the service user to our static uid/gid (NFS + tmpfiles parity).
                  users.users.komga.uid = lib.mkForce cfg.uid;
                  users.groups.komga.gid = lib.mkForce cfg.gid;
                };
            };
        };
      };
  };
}
