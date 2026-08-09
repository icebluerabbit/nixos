_: {
  den.aspects.containers._.tuwunel = {
    aspect-name = "containers.tuwunel";
    includes = [ ];

    provides.to-hosts.nixos =
      { config, lib, ... }:
      let
        hostConfig = config;
      in
      {
        options.containerConfigs.tuwunel =
          (import ../_options.nix {
            inherit lib;
            defaultPort = 6167;
            defaultUid = 5024;
            defaultGid = 5024;
          })
          // {
            serverName = lib.mkOption {
              type = lib.types.str;
              default = "matrix.services.internal";
              description = "Matrix server_name used in MXIDs";
            };
            envFile = lib.mkOption {
              type = lib.types.nullOr lib.types.str;
              default = null;
              description = "Path to env file with TUWUNEL_<USERNAME>_PASS vars for initUsers";
            };
            registrationSharedSecret = lib.mkOption {
              type = lib.types.str;
              description = ''
                Synapse-compatible registration shared secret. The init script uses it to
                create initUsers via the /_synapse/admin/v1/register HMAC API (nonce + mac),
                so passwords go in the request body — never on the server's argv (unlike the
                old --execute bootstrap). LAN-only homeserver.
              '';
            };
            initUsers = lib.mkOption {
              type = lib.types.listOf (
                lib.types.submodule {
                  options = {
                    username = lib.mkOption { type = lib.types.str; };
                    admin = lib.mkOption {
                      type = lib.types.bool;
                      default = false;
                    };
                    avatar = lib.mkOption {
                      type = lib.types.str;
                      default = "";
                      description = "Papirus app-icon name for this user's avatar; empty = generic fallback.";
                    };
                  };
                }
              );
              default = [ ];
              description = "Users to declaratively create (idempotently) via the shared-secret register API.";
            };
            initRooms = lib.mkOption {
              type = lib.types.listOf (
                lib.types.submodule {
                  options = {
                    alias = lib.mkOption { type = lib.types.str; };
                    name = lib.mkOption { type = lib.types.str; };
                    space = lib.mkOption {
                      type = lib.types.bool;
                      default = false;
                      description = "Create as an m.space room (grouping) instead of a normal room.";
                    };
                    parent = lib.mkOption {
                      type = lib.types.str;
                      default = "";
                      description = "Alias (localpart, no # or :server) of the parent space; wires m.space.child/parent state.";
                    };
                    members = lib.mkOption {
                      type = lib.types.listOf lib.types.str;
                      default = [ ];
                    };
                    icon = lib.mkOption {
                      type = lib.types.str;
                      default = "";
                      description = "Papirus app-icon name for the room avatar; empty = use the alias, falling back to a generic icon.";
                    };
                  };
                }
              );
              default = [ ];
              description = "Rooms/spaces to declaratively create, link, and populate after startup";
            };
          };

        config =
          let
            cfg = hostConfig.containerConfigs.tuwunel;
          in
          {
            containers.tuwunel = {
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
                  # Shared secret in a store file → tuwunel reads it via
                  # registration_shared_secret_file (a path in argv, not the secret). The init
                  # script signs the same secret to create users through the HMAC register API.
                  regSecretFile = pkgs.writeText "tuwunel-reg-secret" cfg.registrationSharedSecret;

                  adminUser = lib.findFirst (u: u.admin) null cfg.initUsers;

                  initScript = pkgs.writeTextFile {
                    name = "tuwunel-init.py";
                    text = builtins.readFile ./init.py;
                  };

                  # Room/user avatars: Papirus app SVGs rasterised to PNG at build time (Matrix
                  # clients refuse SVG avatars, and the server thumbnailer is raster-only). Room
                  # icon defaults to the alias (Papirus names line up with the app rooms); missing
                  # names fall back to a generic glyph. init.py uploads these and sets the avatars.
                  papirusApps = "${pkgs.papirus-icon-theme}/share/icons/Papirus/64x64/apps";
                  fallbackSvg = ./avatars/_fallback.svg;
                  avatarEntries =
                    map (r: {
                      key = "room-${r.alias}";
                      icon = if r.icon != "" then r.icon else r.alias;
                    }) cfg.initRooms
                    ++ map (u: {
                      key = "user-${u.username}";
                      icon = u.avatar; # "" → fallback
                    }) cfg.initUsers;
                  avatars = pkgs.runCommand "tuwunel-avatars" { nativeBuildInputs = [ pkgs.librsvg ]; } ''
                    mkdir -p "$out"
                    conv() {
                      local src="${fallbackSvg}"
                      if [ -n "$2" ] && [ -f "${papirusApps}/$2.svg" ]; then
                        src="${papirusApps}/$2.svg"
                      fi
                      rsvg-convert -w 256 -h 256 "$src" -o "$out/$1.png"
                    }
                    ${lib.concatMapStringsSep "\n" (
                      e: "conv ${lib.escapeShellArg e.key} ${lib.escapeShellArg e.icon}"
                    ) avatarEntries}
                  '';

                  # Inject the built PNG path into each room/user record for init.py to upload.
                  roomsWithIcon = map (r: r // { iconFile = "${avatars}/room-${r.alias}.png"; }) cfg.initRooms;
                  usersWithAvatar = map (u: u // { avatarFile = "${avatars}/user-${u.username}.png"; }) cfg.initUsers;
                in
                {
                  imports = [ (import ../_base.nix { inherit lib cfg; }) ];

                  services.matrix-tuwunel = {
                    enable = true;
                    settings.global = {
                      server_name = cfg.serverName;
                      address = [ "0.0.0.0" ];
                      port = [ cfg.port ];
                      new_user_displayname_suffix = "";
                      allow_registration = false;
                      allow_federation = false;
                      trusted_servers = [ ];
                      db_pool_max_workers = 256;
                      # Enables /_synapse/admin/v1/register (shared-secret HMAC) so the init
                      # service can provision initUsers off-argv. Public registration stays off.
                      registration_shared_secret_file = toString regSecretFile;
                    };
                  };

                  networking.hosts = lib.mkIf (cfg.initRooms != [ ]) {
                    "127.0.0.1" = [ cfg.serverName ];
                  };

                  users.users.tuwunel = {
                    uid = lib.mkForce cfg.uid;
                    group = "tuwunel";
                    home = lib.mkForce "/var/lib/tuwunel";
                    isSystemUser = true;
                  };
                  users.groups.tuwunel.gid = lib.mkForce cfg.gid;

                  systemd.services.tuwunel.serviceConfig = {
                    DynamicUser = lib.mkForce false;
                    PrivateUsers = lib.mkForce false;
                    # Clean argv: user provisioning moved to tuwunel-init's shared-secret API
                    # calls, so no --execute (and no passwords) on the long-running process.
                    # Config is still supplied via the module's environment, as before.
                    ExecStart = lib.mkForce (lib.getExe pkgs.matrix-tuwunel);
                  };

                  # One post-startup pass: create/admin the initUsers via the shared-secret
                  # register API (idempotent — already-present users return "in use" and are
                  # skipped), then create/link/populate the rooms as that admin.
                  systemd.services.tuwunel-init =
                    lib.mkIf ((cfg.initUsers != [ ] || cfg.initRooms != [ ]) && adminUser != null)
                      {
                        description = "Provision tuwunel users (shared-secret register) + rooms";
                        after = [ "tuwunel.service" ];
                        wants = [ "tuwunel.service" ];
                        wantedBy = [ "multi-user.target" ];
                        serviceConfig = {
                          Type = "simple";
                          EnvironmentFile = lib.mkIf (cfg.envFile != null) "/run/secrets/tuwunel-env";
                          Environment = "TUWUNEL_REG_SECRET=${cfg.registrationSharedSecret}";
                          ExecStart =
                            let
                              usersJson = builtins.toJSON usersWithAvatar;
                              roomsJson = builtins.toJSON roomsWithIcon;
                            in
                            "${lib.getExe pkgs.python3} ${initScript} ${toString cfg.port} ${cfg.serverName} ${adminUser.username} '${usersJson}' '${roomsJson}'";
                        };
                      };
                };
            };
          };
      };
  };
}
