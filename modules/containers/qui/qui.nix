/*
  qui — modern alternative web UI for qBittorrent (multi-instance)
  https://github.com/autobrr/qui

  Replaces flood. Uses the upstream `services.qui` NixOS module.

  Auth is DISABLED and restricted by source CIDR (authDisabledAllowedCIDRs) — qui
  is a LAN-only convenience UI reached through Caddy; access is gated by the
  network, not a login. The module still requires a session secret (secretFile)
  for at-rest encryption of stored instance passwords.

  The qBittorrent instance is registered declaratively by `qui_provision.py` (a
  post-start oneshot hitting qui's API) — qui has no config-file for instances.
*/
_: {
  den.aspects.containers._.qui = {
    aspect-name = "containers.qui";
    includes = [ ];

    provides.to-hosts.nixos =
      {
        config,
        lib,
        pkgs,
        ...
      }:
      {
        options.containerConfigs.qui =
          (import ../_options.nix {
            inherit lib;
            defaultPort = 7476;
            defaultUid = 5029;
            defaultGid = 5029;
          })
          // {
            secretFile = lib.mkOption {
              type = lib.types.str;
              description = "In-container path to the qui session secret (openssl rand -hex 32, via sops).";
            };
            allowedCIDRs = lib.mkOption {
              type = lib.types.listOf lib.types.str;
              default = [ ];
              description = "Source CIDRs permitted while auth is disabled (qui authDisabledAllowedCIDRs).";
            };
            qbtUrl = lib.mkOption {
              type = lib.types.str;
              description = "qBittorrent WebUI URL auto-registered as a qui instance (e.g. http://10.10.130.5:8080).";
            };
          };

        config = {
          systemd.services."container@qui" = {
            serviceConfig.MemoryHigh = lib.mkIf (
              config.containerConfigs.qui.memoryHigh != null
            ) config.containerConfigs.qui.memoryHigh;
          };

          containers.qui =
            let
              cfg = config.containerConfigs.qui;
              # Resolved from host pkgs so the homelab overlay is in scope; the
              # store path pulls into the container closure via ExecStart.
              quiProvision = lib.getExe pkgs.homelab.qui-provision;
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
                    (import ../_base.nix { inherit lib cfg; })
                  ];

                  services.qui = {
                    enable = true;
                    user = "qui";
                    group = "qui";
                    inherit (cfg) secretFile;
                    # freeform TOML → config.toml. sessionSecret must NOT go here
                    # (module asserts it) — it comes from secretFile.
                    settings = {
                      host = "0.0.0.0";
                      inherit (cfg) port;
                      authDisabled = true;
                      I_ACKNOWLEDGE_THIS_IS_A_BAD_IDEA = true;
                      authDisabledAllowedCIDRs = cfg.allowedCIDRs;
                      checkForUpdates = false; # disable GitHub update polling; version is Nix-pinned
                    };
                  };

                  # Pin UID/GID so the bind-mounted /var/lib/qui (SQLite DB) keeps
                  # stable ownership across nspawn's user-namespace remap.
                  users.users.qui.uid = lib.mkForce cfg.uid;
                  users.groups.qui.gid = lib.mkForce cfg.gid;

                  # nspawn already gives the container its own user namespace; the
                  # module's PrivateUsers=true nests a second one and breaks
                  # StateDirectory/LoadCredential setup (same fix qbt needs).
                  systemd.services.qui.serviceConfig.PrivateUsers = lib.mkForce false;

                  # Register the qBittorrent instance via qui's API (idempotent).
                  # Runs after qui; the localhost source is covered by allowedCIDRs.
                  systemd.services.qui-provision = {
                    description = "Register qBittorrent instance in qui (idempotent, API)";
                    after = [ "qui.service" ];
                    wantedBy = [ "multi-user.target" ];
                    serviceConfig = {
                      Type = "oneshot";
                      ExecStart = "${quiProvision} http://127.0.0.1:${toString cfg.port} ${cfg.qbtUrl} qbittorrent";
                    };
                  };
                };
            };
        };
      };
  };
}
