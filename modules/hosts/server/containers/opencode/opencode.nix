# opencode — host configuration override for opencode AI coding agent container
{ den, ... }:
{
  den.aspects.hosts._.server._.containers._.opencode = {
    aspect-name = "hosts.server.containers.opencode";
    includes = [
      den.aspects.containers._.opencode
      den.aspects.infra._.clients._.container-opencode
      den.aspects.infra._.clients._.container-dns
      den.aspects.infra._.networks._.services
      den.aspects.hosts._.server._.networking._.vlans._.services
      den.aspects.hosts._.server._.security
      den.aspects.infra._.clients._.container-loki
      den.aspects.infra._.clients._.container-prometheus
      den.aspects.infra._.clients._.container-tempo
    ];

    nixos =
      {
        config,
        lib,
        pkgs,
        ...
      }:
      let
        cfg = config.containerConfigs.opencode;
        wsGid = config.users.groups.workspace.gid;
      in
      {
        systemd.services."container@opencode" = {
          after = [ "container-wave-core.target" ];
          before = [ "container-wave-apps.target" ];
        };

        # git-over-ssh to gitea uses this container's own key (appdata → /home/opencode/.ssh)
        containers.opencode.config = {
          # Baseline toolbelt: what agents reach for constantly, always present and
          # needing no network. Anything beyond this is a `nix shell nixpkgs#…` away.
          environment.systemPackages = with pkgs; [
            openssh
            direnv
            ripgrep
            fd
            jq
            curl
            gnumake
          ];

          # The workspace is co-owned with code-server (different uid), so libgit2's
          # ownership check rejects every repo in it — which kills `use flake` in any
          # .envrc. Scoped globs aren't supported, only exact paths or `*`, and the
          # agent clones new repos freely. It already has full write access to this
          # tree, so trusting it grants nothing new.
          programs.git = {
            enable = true;
            config.safe.directory = "*";
          };

          # Shared working tree: same numeric gid host↔container (--private-users=no),
          # so opencode co-writes /var/lib/workspace with code-server and host zms.
          users.groups.workspace.gid = wsGid;
          users.users.opencode.extraGroups = [ "workspace" ];
          programs.ssh.extraConfig = ''
            Host gitea.apps.internal
              User git
              Port 2222
              IdentityFile /home/opencode/.ssh/id_ed25519
              IdentitiesOnly yes
              StrictHostKeyChecking accept-new
          '';
        };

        containerConfigs.opencode = {
          lldpd.enable = true;
          hostBridge = "br-services";
          macAddress = config.infra.clients.container-opencode.mac;
          localAddress = "${config.infra.clients.container-opencode.ip}/${toString config.infra.vlans.services.subnetPrefix}";
          gateway = config.infra.vlans.services.staticAddresses.gateway;
          port = config.infra.clients.container-opencode.ports.default;
          envFile = config.sops.templates."opencode-env".path;
          otlpEndpoint = "http://${config.infra.clients.container-prometheus.ip}:${toString config.infra.clients.container-prometheus.ports.default}/api/v1/otlp";
          nameservers = [
            config.infra.clients.container-dns.ip
            "1.1.1.1"
          ];
          workingDirectory = "/var/lib/workspace";
          settings = {
            # Default model, routed through OpenRouter (single account shared with
            # hermes); key arrives as OPENROUTER_API_KEY via the opencode-env sops
            # template. Clients (openchamber/code-server) may still pick per-session.
            model = "openrouter/anthropic/claude-sonnet-4.6";
            # Skills live beside this module so they are reviewed and version-
            # controlled here; the store copy is read-only to the agent.
            skills.paths = [ "${./skills}" ];
            mcp = {
              filesystem = {
                type = "local";
                enabled = true;
                command = [
                  (lib.getExe pkgs.mcp-server-filesystem)
                  "/var/lib/workspace"
                ];
              };
            };
          };
          bindMounts = {
            "/home/opencode" = {
              hostPath = "/var/lib/appdata/opencode";
              isReadOnly = false;
            };
            "/var/lib/workspace" = {
              hostPath = "/var/lib/workspace";
              isReadOnly = false;
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
          "d /var/lib/appdata/opencode 0770 ${toString cfg.uid} ${toString cfg.gid} - -"
          "Z /var/lib/appdata/opencode 0770 ${toString cfg.uid} ${toString cfg.gid} - -"
        ];
      };
  };
}
