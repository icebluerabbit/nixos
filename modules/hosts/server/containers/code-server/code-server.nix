# code-server — host configuration override for the code-server web IDE container
{ den, ... }:
{
  den.aspects.hosts._.server._.containers._.code-server = {
    aspect-name = "hosts.server.containers.code-server";
    includes = [
      den.aspects.containers._.code-server
      den.aspects.infra._.clients._.container-code-server
      den.aspects.infra._.clients._.container-dns
      den.aspects.infra._.clients._.container-gitea
      den.aspects.infra._.clients._.container-opencode
      den.aspects.infra._.networks._.apps
      den.aspects.infra._.networks._.services # opencode's VLAN, for its staticAddress
      den.aspects.hosts._.server._.networking._.vlans._.apps
      den.aspects.hosts._.server._.security
      den.aspects.infra._.clients._.container-loki
      den.aspects.infra._.clients._.container-prometheus
      den.aspects.infra._.clients._.container-tempo
    ];

    nixos =
      { config, pkgs, ... }:
      let
        cfg = config.containerConfigs.code-server;
        wsGid = config.users.groups.workspace.gid;
        # Forgejo/Gitea integration (clone provider + repositories view). Not in nixpkgs.
        # The generic symlink-into-extensions-dir path does NOT work for this: code-server
        # manages extensions via extensions.json + the Open VSX gallery and "Marks as
        # removed" any symlinked extension not already tracked there (confirmed in the
        # extension-host log). So install it with code-server's OWN installer from the vsix
        # (preStart below) — that registers it in extensions.json, which code-server keeps.
        # Configured via forgejo.instances in settings.json; API token → SecretStorage.
        forgejoVsix = pkgs.fetchurl {
          url = "https://open-vsx.org/api/maxking/forgejo-vscode/0.3.24/file/maxking.forgejo-vscode-0.3.24.vsix";
          hash = "sha256-BXRLPDx+wnmzNBQ4b9zS46hwtmUv/+uu6hqhynKgcjo=";
        };
        # OpenChamber — agent sidebar for opencode. Same vsix-installer path as forgejo
        # above (not in nixpkgs). Attaches to the server-side agent via
        # openchamber.apiUrl in settings.json; empty would spawn a local instance.
        openchamberVsix = pkgs.fetchurl {
          url = "https://open-vsx.org/api/FedaykinDev/openchamber/1.16.1/file/FedaykinDev.openchamber-1.16.1.vsix";
          hash = "sha256-xLhlbifJaXKzXK1wsoNqfzb4ntuL/HAH25XKcQaJzsU=";
        };
      in
      {
        systemd.services."container@code-server" = {
          after = [ "container-wave-core.target" ];
          before = [ "container-wave-apps.target" ];
        };

        # Enroll the container's code-server user in the shared workspace group (same
        # numeric gid as the host; --private-users=no so it matches), so it can write
        # files created by other members (host zms, later opencode).
        containers.code-server.config =
          {
            config,
            pkgs,
            lib,
            ...
          }:
          {
            users.groups.workspace.gid = wsGid;
            users.users.code-server.extraGroups = [ "workspace" ];
            # git-over-ssh to gitea uses this container's own key (appdata → ~/.ssh)
            programs.ssh.extraConfig = ''
              Host gitea.apps.internal
                User git
                Port 2222
                IdentityFile /var/lib/code-server/.ssh/id_ed25519
                IdentitiesOnly yes
                StrictHostKeyChecking accept-new
            '';
            # forgejo API token → SecretStorage via the deprecated-but-honored
            # forgejo.token key. Read from a systemd credential (not a world-readable
            # bind mount) and spliced into the seeded settings.json by the sidecar
            # script below, so the plaintext never enters the Nix store.
            systemd.services.code-server.serviceConfig.LoadCredential = [
              "forgejo-token:/run/secrets/forgejo-token"
            ];

            # Install the vsix-only extensions via code-server's own installer (registers
            # them in extensions.json) — the symlink path gets marked-removed by
            # code-server. --user-data-dir/--extensions-dir are ExecStart-only flags in
            # the nixpkgs module, so the CLI here must be told both explicitly —
            # otherwise it installs into $HOME/.local/share/code-server/extensions,
            # which the running server never reads.
            systemd.services.code-server.preStart = lib.mkAfter ''
              ${lib.getExe config.services.code-server.package} \
                --user-data-dir ${config.services.code-server.userDataDir} \
                --extensions-dir ${config.services.code-server.extensionsDir} \
                --install-extension ${forgejoVsix} --force
              ${lib.getExe config.services.code-server.package} \
                --user-data-dir ${config.services.code-server.userDataDir} \
                --extensions-dir ${config.services.code-server.extensionsDir} \
                --install-extension ${openchamberVsix} --force
              ${pkgs.python3}/bin/python3 ${./inject-forgejo-token.py} \
                ${config.services.code-server.userDataDir}/User/settings.json \
                "$CREDENTIALS_DIRECTORY/forgejo-token"
            '';
          };

        containerConfigs.code-server = {
          lldpd.enable = true;
          hostBridge = "br-apps";
          macAddress = config.infra.clients.container-code-server.mac;
          localAddress = "${config.infra.clients.container-code-server.ip}/${toString config.infra.vlans.apps.subnetPrefix}";
          gateway = config.infra.vlans.apps.staticAddresses.gateway;
          port = config.infra.clients.container-code-server.ports.web;
          nameservers = [
            config.infra.clients.container-dns.ip
            "1.1.1.1"
          ];

          # Editor settings, fully nix-owned; edit this file and rebuild, not the UI.
          settingsFile = (pkgs.formats.json { }).generate "settings.json" (
            import ./_settings.nix { inherit config; }
          );

          # Tier-2 declarative extensions (Open VSX gallery, nix-pinned).
          extensions = with pkgs.vscode-extensions; [
            jnoortheen.nix-ide # nixd client
            golang.go
            rust-lang.rust-analyzer
            mkhl.direnv # injects devshell env into the extension host (LSP-visible)
            charliermarsh.ruff
            # Stock Mocha only: the extension recompiles its theme JSON inside its own
            # (read-only, store-symlinked) dir, so catppuccin.* knobs won't apply here.
            catppuccin.catppuccin-vsc
            # Same read-only caveat: custom file/folder associations won't stick.
            pkief.material-icon-theme
          ];

          # Global fallback language servers for repos without a flake devshell.
          editorPackages = with pkgs; [
            nixd
            gopls
            rust-analyzer
            typescript-language-server
            pyright
            ruff
          ];

          bindMounts = {
            "/var/lib/code-server" = {
              hostPath = "/var/lib/appdata/code-server";
              isReadOnly = false;
            };
            "/var/lib/workspace" = {
              hostPath = "/var/lib/workspace";
              isReadOnly = false;
            };
            # forgejo token: host sops secret → container, read once by the
            # in-container LoadCredential (see containers.code-server.config above).
            "/run/secrets/forgejo-token" = {
              hostPath = config.sops.secrets."gitea-api-token".path;
              isReadOnly = true;
            };
            # Host nix daemon socket → in-container devshell builds (store already shared).
            "/nix/var/nix/daemon-socket" = {
              hostPath = "/nix/var/nix/daemon-socket";
              isReadOnly = true;
            };
          };

          logging = {
            enable = true;
            lokiAddress = "http://${config.infra.clients.container-loki.ip}:${toString config.infra.clients.container-loki.ports.default}";
            prometheusAddress = "http://${config.infra.clients.container-prometheus.ip}:${toString config.infra.clients.container-prometheus.ports.default}/api/v1/write";
            tempoAddress = "http://${config.infra.clients.container-tempo.ip}:${toString config.infra.clients.container-tempo.ports.otlp-http}/v1/traces";
          };
        };

        systemd.tmpfiles.rules = [
          "d /var/lib/appdata/code-server 0770 ${toString cfg.uid} ${toString cfg.gid} - -"
          "Z /var/lib/appdata/code-server 0770 ${toString cfg.uid} ${toString cfg.gid} - -"
          # shared working tree: owned by code-server, group 'workspace' + setgid (2xxx)
          # so new files inherit the group; default ACL grants group rwx on new files
          # regardless of each writer's umask. Host zms + code-server (+ later opencode)
          # co-write it.
          "d /var/lib/workspace 2770 ${toString cfg.uid} ${toString wsGid} - -"
          "a+ /var/lib/workspace - - - - default:group:${toString wsGid}:rwx,group:${toString wsGid}:rwx"
        ];
      };
  };
}
