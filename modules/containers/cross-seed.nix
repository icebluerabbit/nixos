/*
  cross-seed — automatic torrent cross-seeding via Torznab
  https://www.cross-seed.org
*/
_: {
  den.aspects.containers._.cross-seed = {
    aspect-name = "containers.cross-seed";
    includes = [ ];

    provides.to-hosts.nixos =
      { config, lib, ... }:
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
                  imports = [ (import ./_base.nix { inherit lib cfg; }) ];

                  services.cross-seed = {
                    enable = true;
                    user = "cross-seed";
                    group = "cross-seed";
                    inherit (cfg) configDir settings settingsFile;
                  };

                  users.users.cross-seed.uid = lib.mkForce cfg.uid;
                  users.groups.cross-seed.gid = lib.mkForce cfg.gid;
                };
            };
        };
      };
  };
}
