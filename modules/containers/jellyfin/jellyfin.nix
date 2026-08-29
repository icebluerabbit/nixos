/*
  jellyfin — volunteer-built media solution/server
  https://jellyfin.org
*/
{ inputs, ... }:
{
  den.aspects.containers._.jellyfin = {
    aspect-name = "containers.jellyfin";
    includes = [ ];

    provides.to-hosts.nixos =
      { config, lib, ... }:
      let
        hostConfig = config;
      in
      {
        options.containerConfigs.jellyfin =
          (import ../_options.nix {
            inherit lib;
            defaultPort = 8096;
            defaultUid = 5009;
            defaultGid = 5009;
          })
          // {
            acceleration = {
              enable = lib.mkOption {
                type = lib.types.bool;
                default = false;
                description = "Enable hardware graphics acceleration inside the container";
              };
              devices = lib.mkOption {
                type = lib.types.listOf lib.types.str;
                default = [ ];
                description = "List of DRI/GPU device paths to pass into the container";
              };
              extraPackages = lib.mkOption {
                type = lib.types.listOf lib.types.package;
                default = [ ];
                description = "Extra graphics packages to install inside the container";
              };
              libvaDriverName = lib.mkOption {
                type = lib.types.nullOr lib.types.str;
                default = null;
                description = "Value for the LIBVA_DRIVER_NAME environment variable";
              };
            };
            envFile = lib.mkOption {
              type = lib.types.nullOr lib.types.path;
              default = null;
              description = "Host path to the env file with JELLYFIN_API_KEY, mounted and loaded by configuratarr";
            };
            configuratarr = lib.mkOption {
              default = { };
              description = "Declarative configuratarr sync for this Jellyfin instance";
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
          systemd.services."container@jellyfin" = {
            serviceConfig.MemoryHigh = lib.mkIf (
              config.containerConfigs.jellyfin.memoryHigh != null
            ) config.containerConfigs.jellyfin.memoryHigh;
          };

          containers.jellyfin =
            let
              cfg = hostConfig.containerConfigs.jellyfin;
            in
            {
              inherit (cfg) autoStart;
              ephemeral = true;
              timeoutStartSec = "5min";
              privateNetwork = true;
              inherit (cfg) hostBridge;
              localMacAddress = cfg.macAddress;
              inherit (cfg) localAddress;
              allowedDevices = lib.optionals cfg.acceleration.enable (
                map (dev: {
                  node = dev;
                  modifier = "rw";
                }) cfg.acceleration.devices
              );
              bindMounts =
                cfg.bindMounts
                // (lib.optionalAttrs cfg.acceleration.enable (
                  lib.genAttrs cfg.acceleration.devices (dev: {
                    hostPath = dev;
                    isReadOnly = false;
                  })
                ))
                // (lib.optionalAttrs (cfg.envFile != null) {
                  "/run/secrets/jellyfin-env" = {
                    hostPath = cfg.envFile;
                    isReadOnly = true;
                  };
                });

              config =
                {
                  lib,
                  ...
                }:
                {
                  imports = [
                    (import ../_base.nix { inherit lib cfg; })
                    inputs.configuratarr.nixosModules.default
                  ];

                  hardware.graphics = {
                    enable = cfg.acceleration.enable;
                    extraPackages = lib.mkIf cfg.acceleration.enable cfg.acceleration.extraPackages;
                  };

                  environment.sessionVariables =
                    lib.mkIf (cfg.acceleration.enable && cfg.acceleration.libvaDriverName != null)
                      {
                        LIBVA_DRIVER_NAME = cfg.acceleration.libvaDriverName;
                      };

                  services.jellyfin = {
                    enable = true;
                    openFirewall = false;
                    user = "jellyfin";
                    group = "jellyfin";
                  };

                  # Declarative config sync. The configuratarr module orders after
                  # jellyfin.service; waitForHealthy polls the authenticated
                  # /System/Info (with JELLYFIN_API_KEY) before syncing.
                  services.configuratarr = lib.mkIf cfg.configuratarr.enable {
                    enable = true;
                    inherit (cfg.configuratarr)
                      settings
                      prune
                      waitForHealthy
                      waitTimeout
                      ;
                  };

                  # Load the shared jellyfin-env so ${env.JELLYFIN_API_KEY} resolves
                  # against the same key stored in sops that other services use.
                  systemd.services.configuratarr = lib.mkIf cfg.configuratarr.enable {
                    serviceConfig.EnvironmentFile = lib.mkIf (cfg.envFile != null) "/run/secrets/jellyfin-env";
                  };

                  # Jellyfin validates TranscodingTempPath server-side and rejects a
                  # path that does not already exist ("<path> does not exist", surfaced
                  # as HTTP 404), so a configuratarr sync that relocates the transcode
                  # cache fails unless the directory is pre-created. tmpfiles runs from
                  # sysinit.target, well before configuratarr.service.
                  systemd.tmpfiles.rules =
                    let
                      transcodePath = lib.attrByPath [
                        "jellyfin"
                        "encoding_options"
                        "transcoding_temp_path"
                      ] null cfg.configuratarr.settings;
                    in
                    lib.optional (
                      cfg.configuratarr.enable && transcodePath != null
                    ) "d ${transcodePath} 0700 jellyfin jellyfin - -";

                  users = {
                    users.jellyfin = {
                      uid = lib.mkForce cfg.uid;
                      isSystemUser = lib.mkForce false;
                      isNormalUser = true;
                      extraGroups = lib.optionals cfg.acceleration.enable [
                        "video"
                        "render"
                      ];
                    };
                    groups = {
                      jellyfin.gid = lib.mkForce cfg.gid;
                      # Inherit video and render group GIDs from host dynamically if defined
                      video.gid = lib.mkIf (hostConfig.users.groups.video.gid != null) (
                        lib.mkForce hostConfig.users.groups.video.gid
                      );
                      render.gid = lib.mkIf (hostConfig.users.groups.render.gid != null) (
                        lib.mkForce hostConfig.users.groups.render.gid
                      );
                    };
                  };

                };
            };
        };
      };
  };
}
