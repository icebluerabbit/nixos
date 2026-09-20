/*
  cross-seed — automatic torrent cross-seeding via Torznab
  https://www.cross-seed.org
*/
_: {
  den.aspects.containers._.cross-seed = {
    aspect-name = "containers.cross-seed";
    includes = [ ];

    provides.to-hosts.nixos =
      {
        config,
        lib,
        pkgs,
        ...
      }:
      let
        # Resolved in the HOST's package set. pkgs inside a container body is
        # the container's own, which carries no overlays — referencing
        # pkgs.homelab there fails with "attribute 'homelab' missing". The
        # store path is what crosses the boundary.
        apiExe = lib.getExe pkgs.homelab.managarr-crossseed;
      in
      {
        options.containerConfigs.cross-seed =
          (import ./_options.nix {
            inherit lib;
            defaultPort = 2468;
            # Share qBittorrent's UID/GID: fs.protected_hardlinks only lets a
            # user hardlink files it owns (or has rw on), so injection must run
            # as qbt's owner to link its download data into linkDirs.
            defaultUid = 5019;
            defaultGid = 5019;
          })
          // {
            configDir = lib.mkOption {
              type = lib.types.str;
              default = "/var/lib/cross-seed";
              description = "cross-seed config/state directory";
            };
            settings = lib.mkOption {
              type = lib.types.attrs;
              default = { };
              description = "Non-secret cross-seed settings (rendered into the Nix store)";
            };
            settingsFile = lib.mkOption {
              type = lib.types.nullOr lib.types.path;
              default = null;
              description = "In-container path to a JSON secrets file merged over settings at runtime";
            };
            apiPort = lib.mkOption {
              type = lib.types.port;
              default = 9910;
              description = ''
                Port for managarr-crossseed, the read-only sidecar serving
                cross-seed's decision log.

                9910 on every managarr-* sidecar. Each container has its own
                IP, so one number is unambiguous everywhere and there is no
                per-service arithmetic to get wrong.

                cross-seed records every candidate it considered and its verdict
                in SQLite and exposes none of it: its HTTP API is /api/ping,
                /api/announce and /api/webhook, and its webhook fires only on a
                match. The rejections — four fifths of the record, and the only
                answer to "why was this not cross-seeded" — are reachable only
                from the file.

                The sidecar runs HERE, beside that file, rather than consumers
                bind-mounting another service's database: a mount needs a
                matching uid and makes the consumer depend on a schema upstream
                is free to change.
              '';
            };
          };

        config = {
          containers.cross-seed =
            let
              cfg = config.containerConfigs.cross-seed;
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
                { lib, ... }:
                {
                  imports = [
                    (import ./_base.nix { inherit lib cfg; })
                    # The read-only sidecar. Runs as cross-seed's own user, so
                    # it reads the database with exactly the access cross-seed
                    # has and no more — which is the whole reason it lives in
                    # this container rather than in the consumer's.
                    {
                      networking.firewall.allowedTCPPorts = [ cfg.apiPort ];

                      systemd.services.managarr-crossseed = {
                        description = "Serve cross-seed's decision log to managarr";
                        after = [ "cross-seed.service" ];
                        wantedBy = [ "multi-user.target" ];
                        wants = [ "cross-seed.service" ];

                        script =
                          "exec "
                          + lib.concatStringsSep " " (
                            [
                              apiExe
                              "--addr :${toString cfg.apiPort}"
                              "--metrics.addr 127.0.0.1:9909"
                            ]
                            ++ [ "--db-path ${cfg.configDir}/cross-seed.db" ]
                          );

                        serviceConfig = {
                          User = "cross-seed";
                          Group = "cross-seed";
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
                  ];

                  services.cross-seed = {
                    enable = true;
                    user = "cross-seed";
                    group = "cross-seed";
                    inherit (cfg) configDir settings settingsFile;
                  };

                  users.users.cross-seed.uid = lib.mkForce cfg.uid;
                  users.groups.cross-seed.gid = lib.mkForce cfg.gid;

                  # The database is opened read-only and the snapshot copy goes
                  # to the unit's private /tmp, so this directory never needs to
                  # be writable by the sidecar.
                  systemd.services.managarr-crossseed.serviceConfig.ReadOnlyPaths = [
                    cfg.configDir
                  ];
                };
            };
        };
      };
  };
}
