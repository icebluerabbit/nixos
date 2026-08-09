# beets — CD-rip tagger: AcoustID fingerprint + MusicBrainz + ReplayGain +
# cover art, moving finished albums into the Lidarr-owned
# /mnt/media/library/music tree. Rips are not downloads, so they enter the
# stack here at the library layer instead of through the qbt/slskd grab flow.
_: {
  den.aspects.containers._.beets = {
    aspect-name = "containers.beets";
    includes = [ ];

    provides.to-hosts.nixos =
      { config, lib, ... }:
      let
        hostConfig = config;
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
            importInterval = lib.mkOption {
              type = lib.types.str;
              default = "10min";
              description = "OnUnitActiveSec cadence for the import timer";
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
          };

        config.containers.beets =
          let
            cfg = hostConfig.containerConfigs.beets;
          in
          {
            inherit (cfg) autoStart;
            ephemeral = true;
            privateNetwork = true;
            inherit (cfg) hostBridge localAddress bindMounts;
            localMacAddress = cfg.macAddress;

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
                  directory = "/mnt/media/library/music";
                  library = "/var/lib/beets/library.db";
                  plugins = [
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
                  };
                };
                configYaml = (pkgs.formats.yaml { }).generate "beets-config.yaml" (
                  lib.recursiveUpdate structuralSettings cfg.settings
                );

                beetEnv = {
                  BEETSDIR = "/var/lib/beets";
                  HOME = "/var/lib/beets";
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
                imports = [ (import ../_base.nix { inherit lib cfg; }) ];

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
                    description = "beets: import new rips from the inbox";
                    serviceConfig.Type = "oneshot";
                    serviceConfig.ExecStart = "${beetsPkg}/bin/beet -c ${configYaml} import -q ${cfg.importDir}";
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
