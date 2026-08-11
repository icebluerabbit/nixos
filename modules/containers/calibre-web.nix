# calibre-web — web reader/library over a Calibre database, with a declarative
# ingest watcher replacing Calibre-Web-Automated's auto-ingest (LL → ingest → library).
# https://github.com/janeczku/calibre-web
_: {
  den.aspects.containers._.calibre-web = {
    aspect-name = "containers.calibre-web";
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
        options.containerConfigs.calibre-web =
          (import ./_options.nix {
            inherit lib;
            defaultPort = 8083;
            defaultUid = 5031;
            defaultGid = 5031;
          })
          // {
            libraryDir = lib.mkOption {
              type = lib.types.str;
              description = "Calibre library directory (holds metadata.db)";
            };
            ingestDir = lib.mkOption {
              type = lib.types.str;
              description = "Watch folder; new book files here are converted + added to the library";
            };
            dataDir = lib.mkOption {
              type = lib.types.str;
              default = "/var/lib/calibre-web";
              description = "Where Calibre-Web stores app.db (local, not the NFS library)";
            };
          };

        config = {
          systemd.services."container@calibre-web" = {
            serviceConfig.MemoryHigh = lib.mkIf (
              config.containerConfigs.calibre-web.memoryHigh != null
            ) config.containerConfigs.calibre-web.memoryHigh;
          };

          containers.calibre-web =
            let
              cfg = hostConfig.containerConfigs.calibre-web;
              # Resolved from host pkgs (the homelab overlay is not in the
              # container's nixpkgs). calibre itself is NOT in the package's
              # closure — both binaries call calibredb/ebook-convert off PATH,
              # which the units below supply from the container's own nixpkgs.
              calibreTools = pkgs.homelab.calibre-tools;
              libInitCmd = lib.escapeShellArgs [
                "${calibreTools}/bin/calibre-library-init"
                "--library"
                cfg.libraryDir
              ];
              ingestCmd = lib.escapeShellArgs [
                "${calibreTools}/bin/calibre-ingest"
                "--library"
                cfg.libraryDir
                "--ingest"
                cfg.ingestDir
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
              inherit (cfg) bindMounts;

              config =
                { pkgs, lib, ... }:
                {
                  imports = [
                    (import ./_base.nix { inherit lib cfg; })
                  ];

                  services.calibre-web = {
                    enable = true;
                    # nixpkgs' py3.14 bump breaks calibre-web's *check* phases only:
                    #  - `scholarly` (a nativeCheckInput) pulls pip-chill, which imports
                    #    pkg_resources (gone from setuptools 82 on 3.14);
                    #  - the runtime-deps check trips on chardet/certifi upper bounds.
                    # Both are build-time only — disable checks + relax the two bounds.
                    package = pkgs.calibre-web.overridePythonAttrs (o: {
                      doCheck = false;
                      pythonRelaxDeps = (o.pythonRelaxDeps or [ ]) ++ [
                        "chardet"
                        "certifi"
                      ];
                    });
                    listen = {
                      ip = "0.0.0.0"; # container is ipv4-only; module default ::1 is unreachable
                      inherit (cfg) port;
                    };
                    inherit (cfg) dataDir;
                    options = {
                      calibreLibrary = cfg.libraryDir;
                      enableBookConversion = true; # wires ebook-convert into app.db
                      enableBookUploading = true;
                    };
                  };

                  # Pin the service user to our static uid/gid (NFS + tmpfiles parity).
                  users.users.calibre-web.uid = lib.mkForce cfg.uid;
                  users.groups.calibre-web.gid = lib.mkForce cfg.gid;

                  # calibre-web's ExecStartPre hard-fails if metadata.db is missing —
                  # it never creates a library. Seed an empty one on first boot.
                  systemd = {
                    services = {
                      calibre-web-libinit = {
                        description = "Create an empty Calibre library if none exists";
                        before = [ "calibre-web.service" ];
                        requiredBy = [ "calibre-web.service" ];
                        path = [ pkgs.calibre ];
                        serviceConfig = {
                          Type = "oneshot";
                          User = "calibre-web";
                          Group = "calibre-web";
                          RemainAfterExit = true;
                          ExecStart = libInitCmd;
                        };
                      };
                      calibre-web.after = [ "calibre-web-libinit.service" ];

                      calibre-web-ingest = {
                        description = "Convert + import books dropped in the ingest folder";
                        after = [ "calibre-web-libinit.service" ];
                        serviceConfig = {
                          Type = "oneshot";
                          User = "calibre-web";
                          Group = "calibre-web";
                          ExecStartPre = "${pkgs.coreutils}/bin/mkdir -p ${lib.escapeShellArg cfg.ingestDir}";
                          ExecStart = ingestCmd;
                        };
                        path = [
                          pkgs.calibre
                          pkgs.coreutils
                          pkgs.findutils
                        ];
                      };
                    };

                    # Declarative auto-ingest (the "Automated" half of CWA): convert
                    # non-epub → epub and add to the library, then remove the source.
                    # Polled by a timer, NOT a systemd.path — inotify does not deliver
                    # events on NFS, and the ingest dir lives on the NFS media array.
                    timers.calibre-web-ingest = {
                      description = "Poll the Calibre ingest folder";
                      wantedBy = [ "timers.target" ];
                      timerConfig = {
                        OnBootSec = "1min";
                        OnUnitActiveSec = "30s";
                      };
                    };
                  };
                };
            };
        };
      };
  };
}
