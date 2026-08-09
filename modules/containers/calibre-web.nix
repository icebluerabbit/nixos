# calibre-web — web reader/library over a Calibre database, with a declarative
# ingest watcher replacing Calibre-Web-Automated's auto-ingest (LL → ingest → library).
# https://github.com/janeczku/calibre-web
_: {
  den.aspects.containers._.calibre-web = {
    aspect-name = "containers.calibre-web";
    includes = [ ];

    provides.to-hosts.nixos =
      { config, lib, ... }:
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
                        serviceConfig = {
                          Type = "oneshot";
                          User = "calibre-web";
                          Group = "calibre-web";
                          RemainAfterExit = true;
                        };
                        script = ''
                          set -eu
                          lib=${lib.escapeShellArg cfg.libraryDir}
                          if [ ! -f "$lib/metadata.db" ]; then
                            mkdir -p "$lib"
                            # calibredb initialises an empty library db on first access.
                            ${pkgs.calibre}/bin/calibredb --with-library="$lib" list >/dev/null 2>&1 || true
                            if [ ! -f "$lib/metadata.db" ]; then
                              ${pkgs.calibre}/bin/calibredb --with-library="$lib" restore_database --really-do-it >/dev/null 2>&1 || true
                            fi
                          fi
                        '';
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
                        };
                        path = [
                          pkgs.calibre
                          pkgs.coreutils
                          pkgs.findutils
                        ];
                        script = ''
                          set -u
                          lib=${lib.escapeShellArg cfg.libraryDir}
                          ingest=${lib.escapeShellArg cfg.ingestDir}
                          shopt -s nullglob nocaseglob 2>/dev/null || true
                          find "$ingest" -type f ! -name '.*' | while read -r f; do
                            # skip files still being written (size changing)
                            s1=$(stat -c %s "$f" 2>/dev/null || echo x)
                            sleep 2
                            s2=$(stat -c %s "$f" 2>/dev/null || echo y)
                            [ "$s1" = "$s2" ] || continue

                            ext=''${f##*.}; ext=''${ext,,}
                            add="$f"; tmp=""
                            if [ "$ext" != "epub" ]; then
                              tmp="$(mktemp --suffix=.epub)"
                              if ebook-convert "$f" "$tmp" >/dev/null 2>&1; then
                                add="$tmp"
                              else
                                add="$f"; rm -f "$tmp"; tmp=""
                              fi
                            fi
                            if calibredb --with-library="$lib" add "$add" >/dev/null 2>&1; then
                              rm -f "$f"
                            fi
                            [ -n "$tmp" ] && rm -f "$tmp" || true
                          done
                        '';
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
