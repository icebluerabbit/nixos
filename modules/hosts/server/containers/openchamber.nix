# openchamber — host configuration override for openchamber web UI container
{ den, ... }:
{
  den.aspects.hosts._.server._.containers._.openchamber = {
    aspect-name = "hosts.server.containers.openchamber";
    includes = [
      den.aspects.containers._.openchamber
      den.aspects.infra._.clients._.container-openchamber
      den.aspects.infra._.clients._.container-dns
      den.aspects.infra._.networks._.apps
      den.aspects.hosts._.server._.networking._.vlans._.apps
      den.aspects.hosts._.server._.security
      den.aspects.infra._.clients._.container-opencode
      den.aspects.infra._.clients._.container-loki
      den.aspects.infra._.clients._.container-prometheus
      den.aspects.infra._.clients._.container-tempo
    ];

    nixos =
      { config, pkgs, ... }:
      let
        cfg = config.containerConfigs.openchamber;
        wsGid = config.users.groups.workspace.gid;
      in
      {
        systemd.services."container@openchamber" = {
          after = [ "container-wave-core.target" ];
          before = [ "container-wave-apps.target" ];
        };

        # git-over-ssh to gitea uses this container's own key (appdata → /var/lib/openchamber/.ssh)
        containers.openchamber.config = {
          environment.systemPackages = [ pkgs.openssh ];
          # Shared working tree: same numeric gid host↔container (--private-users=no),
          # so openchamber co-writes /var/lib/workspace with opencode and code-server.
          users.groups.workspace.gid = wsGid;
          users.users.openchamber.extraGroups = [ "workspace" ];
          programs.ssh.extraConfig = ''
            Host gitea.apps.internal
              User git
              Port 2222
              IdentityFile /var/lib/openchamber/.ssh/id_ed25519
              IdentitiesOnly yes
              StrictHostKeyChecking accept-new
          '';
        };

        containerConfigs.openchamber = {
          lldpd.enable = true;
          hostBridge = "br-apps";
          macAddress = config.infra.clients.container-openchamber.mac;
          localAddress = "${config.infra.clients.container-openchamber.ip}/${toString config.infra.vlans.apps.subnetPrefix}";
          gateway = config.infra.vlans.apps.staticAddresses.gateway;
          port = config.infra.clients.container-openchamber.ports.default;
          opencodeHost = "http://${config.infra.clients.container-opencode.ip}";
          opencodePort = config.infra.clients.container-opencode.ports.default;
          nameservers = [
            config.infra.clients.container-dns.ip
            "1.1.1.1"
          ];
          settings = {
            homeDirectory = "/var/lib/workspace";
            # id is "path_" + base64 of the path, openchamber's own scheme
            projects = [
              {
                id = "path_L3Zhci9saWIvd29ya3NwYWNl";
                path = "/var/lib/workspace";
                addedAt = 0;
                lastOpenedAt = 0;
              }
            ];
            activeProjectId = "path_L3Zhci9saWIvd29ya3NwYWNl";
          };
          bindMounts = {
            "/var/lib/openchamber" = {
              hostPath = "/var/lib/appdata/openchamber";
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
          "d /var/lib/appdata/openchamber 0770 ${toString cfg.uid} ${toString cfg.gid} - -"
          "Z /var/lib/appdata/openchamber 0770 ${toString cfg.uid} ${toString cfg.gid} - -"
        ];
      };
  };
}
