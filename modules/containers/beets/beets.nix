# beets — CD-rip tagger: AcoustID fingerprint + MusicBrainz + ReplayGain +
# cover art. beets tags a rip and writes it to an output directory, then hands
# it to Lidarr, which imports it exactly as it imports a finished download.
#
# Rips therefore rejoin the ordinary grab flow at the import step rather than
# being written into the library directly. Lidarr is the sole writer into the
# library, so a rip and a torrent are named and laid out identically and the
# two cannot drift apart.
_: {
  den.aspects.containers._.beets = {
    aspect-name = "containers.beets";
    includes = [ ];

    provides.to-hosts.nixos =
      {
        config,
        lib,
        pkgs,
        ...
      }:
      let
        # Resolved in the HOST's package set. pkgs inside a container
        # body is the container's own and carries no overlays, so
        # pkgs.homelab there fails with "attribute 'homelab' missing".
        # The store path is what crosses the boundary.
        managarrSidecarExe = lib.getExe pkgs.homelab.managarr-beets;
        hostConfig = config;
        # Resolved from host pkgs so the homelab overlay is in scope; the
        # container's own pkgs has no `homelab` attribute.
        lidarrImportExe = lib.getExe pkgs.homelab.beets-lidarr-import;
      in
      {
        options.containerConfigs.beets =
          (import ../_options.nix {
            inherit lib;
            defaultPort = 8337;
            defaultUid = 5022;
            defaultGid = 5022;
          })
          // {
            importDir = lib.mkOption {
              type = lib.types.str;
              default = "/mnt/media/inbox";
              description = "Directory the import timer scans for freshly-ripped albums";
            };
            outputDir = lib.mkOption {
              type = lib.types.str;
              default = "/mnt/media/complete/rips";
              description = ''
                Where beets writes tagged albums, and where they stay. Lidarr
                links them into the library from here, exactly as it links a
                finished download out of its own complete directory — so this
                is the retained copy of the rip, not a staging area.

                beets must never write into the library itself, or there would
                be two tools naming files with no way to keep them agreeing.
              '';
            };
            importInterval = lib.mkOption {
              type = lib.types.str;
              default = "10min";
              description = "OnUnitActiveSec cadence for the import timer";
            };
            lidarrUrl = lib.mkOption {
              type = lib.types.nullOr lib.types.str;
              default = null;
              description = ''
                Base URL of the Lidarr that imports finished albums, e.g.
                "http://10.10.120.5:8686". Null disables the handoff, leaving
                albums in outputDir for a manual import.
              '';
            };
            lidarrImportMode = lib.mkOption {
              type = lib.types.enum [
                "Copy"
                "Move"
                "Auto"
              ];
              default = "Copy";
              description = ''
                How Lidarr takes the album out of outputDir. Copy keeps the rip
                in place; with Lidarr's copyUsingHardlinks on a shared
                filesystem that links rather than duplicates, so the retained
                copy costs no extra space. Move hands the files over and leaves
                nothing behind.
              '';
            };
            envFile = lib.mkOption {
              type = lib.types.nullOr lib.types.path;
              default = null;
              description = ''
                Env file supplying LIDARR_API_KEY to the import service. Comes
                from a sops template on the host.
              '';
            };
            settings = lib.mkOption {
              type = lib.types.attrs;
              default = { };
              description = ''
                beets config.yaml as a Nix attrset, deep-merged over the
                container-coupled structural defaults (directory, library, web
                host/port, plugin list). This is where import behaviour, path
                formats, and per-plugin tuning live — set from the host override.
              '';
            };
            apiPort = lib.mkOption {
              type = lib.types.port;
              default = 9910;
              description = ''
                Port for managarr-beets, the sidecar serving this service's
                records to managarr.

                9910 on every managarr-* sidecar. Each container has its own
                IP, so one number is unambiguous everywhere and there is no
                per-service arithmetic to get wrong.
              '';
            };
            managarrUrl = lib.mkOption {
              type = lib.types.str;
              default = "";
              description = ''
                managarr base URL the sidecar relays events to; empty disables
                relaying and leaves the read half working.
              '';
            };
          };

        config.containers.beets =
          let
            cfg = hostConfig.containerConfigs.beets;
          in
          {
            inherit (cfg) autoStart;
            ephemeral = true;
            privateNetwork = true;
            inherit (cfg) hostBridge localAddress;
            localMacAddress = cfg.macAddress;

            bindMounts =
              cfg.bindMounts
              // (lib.optionalAttrs (cfg.envFile != null) {
                "/run/secrets/beets-env" = {
                  hostPath = cfg.envFile;
                  isReadOnly = true;
                };
              });

            config =
              { pkgs, lib, ... }:
              let
                # pkgs.beets is `toPythonApplication python3Packages.beets`; the
                # plugin selection lives on the python module, so override there
                # and re-wrap into a runnable application.
                beetsPkg = pkgs.python3Packages.toPythonApplication (
                  (pkgs.python3Packages.beets.override {
                    pluginOverrides = {
                      web.enable = true;
                      chroma.enable = true;
                      fetchart.enable = true;
                      embedart.enable = true;
                      replaygain.enable = true;
                      lastgenre.enable = true;
                      scrub.enable = true;
                      lyrics.enable = true; # fetch + embed (synced LRCLIB) lyrics
                    };
                  }).overridePythonAttrs
                    (old: {
                      # filetote is a third-party beetsplug plugin (not in beets'
                      # built-in set), so add it to the python env rather than via
                      # pluginOverrides. Carries whipper's rip sidecars
                      # (.cue/.log/.m3u/.toc) into the library on import.
                      #
                      # doCheck = false: filetote's test phase pulls beets-audible,
                      # which fails to build in this nixpkgs pin (wants uv_build
                      # <0.11 but the pin ships >=0.11). We only need filetote at
                      # runtime, so skip its tests to drop that broken dep.
                      propagatedBuildInputs = (old.propagatedBuildInputs or [ ]) ++ [
                        (pkgs.python3Packages.beets-filetote.overridePythonAttrs (_: {
                          doCheck = false;
                        }))
                      ];
                    })
                );
                # replaygain's ffmpeg backend and embedart's resizing need these on PATH;
                # chromaprint (fpcalc) is wrapped in by the chroma plugin. flac + mp3val back
                # the badfiles integrity check (flac -t / mp3val on each imported rip).
                runtimePath = lib.makeBinPath [
                  pkgs.ffmpeg
                  pkgs.imagemagick
                  pkgs.flac
                  pkgs.mp3val
                ];

                # Container-coupled invariants: these track the container's port,
                # bind-mount layout, and the plugin set compiled into beetsPkg, so
                # they live here rather than in the host's `settings`. The host's
                # settings deep-merge on top (recursiveUpdate) for everything else.
                structuralSettings = {
                  directory = cfg.outputDir;
                  library = "/var/lib/beets/library.db";
                  plugins = [
                    "hook"
                    "chroma"
                    "fetchart"
                    "embedart"
                    "replaygain"
                    "lastgenre"
                    "scrub"
                    "web"
                    "lyrics"
                    "ftintitle"
                    "badfiles"
                    "fromfilename"
                    "mbsync"
                    "missing"
                    "duplicates"
                    "edit"
                    "info"
                    "filetote"
                  ];
                  import.log = "/var/lib/beets/import.log";
                  web = {
                    host = "0.0.0.0";
                    inherit (cfg) port;
                    # Serve each item's path. Off by default, and without it
                    # every item comes back with "path": null — which makes the
                    # API unusable for anything that has to match a tagged item
                    # against a file, since the path is the only thing beets and
                    # the library have in common.
                    include_paths = true;
                    # Read-only is already the default; stated because this
                    # endpoint is now a dependency of another service, and a
                    # writable library API reachable from the VLAN is a
                    # different thing entirely.
                    readonly = true;
                  };
                }
                // lib.optionalAttrs (cfg.lidarrUrl != null) {
                  # Fires once per album, after beets has finished writing it,
                  # so Lidarr never sees a half-written directory.
                  hook.hooks = [
                    {
                      event = "album_imported";
                      command = "${lidarrImportExe} {album.path}";
                    }
                  ];
                };
                configYaml = (pkgs.formats.yaml { }).generate "beets-config.yaml" (
                  lib.recursiveUpdate structuralSettings cfg.settings
                );

                beetEnv = {
                  BEETSDIR = "/var/lib/beets";
                  HOME = "/var/lib/beets";
                }
                // lib.optionalAttrs (cfg.lidarrUrl != null) {
                  LIDARR_URL = cfg.lidarrUrl;
                  LIDARR_IMPORT_MODE = cfg.lidarrImportMode;
                };
                svcCommon = {
                  path = [
                    beetsPkg
                    runtimePath
                  ];
                  environment = beetEnv;
                  serviceConfig = {
                    User = "beets";
                    Group = "beets";
                  };
                };
              in
              {
                imports = [
                  {
                    networking.firewall.allowedTCPPorts = [ cfg.apiPort ];

                    systemd.services.managarr-beets = {
                      description = "Serve beets' library to managarr";
                      after = [ "beets-web.service" ];
                      wantedBy = [ "multi-user.target" ];
                      wants = [ "beets-web.service" ];

                      script =
                        "exec "
                        + lib.concatStringsSep " " (
                          [
                            managarrSidecarExe
                            "--addr :${toString cfg.apiPort}"
                            "--metrics.addr 127.0.0.1:9909"
                          ]
                          ++ lib.optional (cfg.managarrUrl != "") "--managarr-url ${cfg.managarrUrl}"
                          ++ [ "--upstream-url http://127.0.0.1:8337" ]
                        );

                      serviceConfig = {
                        User = "beets";
                        Group = "beets";
                        Restart = "on-failure";
                        RestartSec = 5;
                        ProtectSystem = "strict";
                        ProtectHome = true;
                        PrivateDevices = true;
                        PrivateTmp = true;
                        NoNewPrivileges = true;
                        RestrictAddressFamilies = [
                          "AF_INET"
                          "AF_INET6"
                        ];
                      };
                    };
                  }
                  (import ../_base.nix { inherit lib cfg; })
                ];

                environment.systemPackages = [
                  beetsPkg
                ];

                users.users.beets = {
                  uid = lib.mkForce cfg.uid;
                  group = "beets";
                  isSystemUser = true;
                  home = "/var/lib/beets";
                };
                users.groups.beets.gid = lib.mkForce cfg.gid;

                systemd = {
                  # Expose the generated config at $BEETSDIR/config.yaml so manual
                  # `beet` runs (interactive import of skipped albums) find it with
                  # no -c flag: `sudo -u beets BEETSDIR=/var/lib/beets beet import …`.
                  tmpfiles.rules = [
                    "L+ /var/lib/beets/config.yaml - - - - ${configYaml}"
                  ];

                  services.beets-web = lib.recursiveUpdate svcCommon {
                    description = "beets web UI";
                    wantedBy = [ "multi-user.target" ];
                    after = [ "network.target" ];
                    serviceConfig.ExecStart = "${beetsPkg}/bin/beet -c ${configYaml} web";
                    serviceConfig.Restart = "on-failure";
                  };
                  services.beets-import = lib.recursiveUpdate svcCommon {
                    description = "beets: tag new rips and hand them to Lidarr";
                    serviceConfig = {
                      Type = "oneshot";
                      ExecStart = "${beetsPkg}/bin/beet -c ${configYaml} import -q ${cfg.importDir}";
                      # Supplies LIDARR_API_KEY to the hook, which inherits the
                      # service environment.
                      EnvironmentFile = lib.mkIf (cfg.envFile != null) "/run/secrets/beets-env";
                    };
                  };

                  timers.beets-import = {
                    description = "poll ${cfg.importDir} for new rips";
                    wantedBy = [ "timers.target" ];
                    timerConfig = {
                      OnBootSec = "5min";
                      OnUnitActiveSec = cfg.importInterval;
                      Persistent = true;
                    };
                  };
                };
              };
          };
      };
  };
}
