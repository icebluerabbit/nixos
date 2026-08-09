# thelounge — host configuration override for thelounge container
{ den, ... }:
{
  den.aspects.hosts._.server._.containers._.thelounge = {
    aspect-name = "hosts.server.containers.thelounge";
    includes = [
      den.aspects.containers._.thelounge
      den.aspects.infra._.clients._.container-thelounge
      den.aspects.infra._.clients._.container-lldap
      den.aspects.infra._.clients._.container-dns
      den.aspects.infra._.networks._.apps
      den.aspects.hosts._.server._.security
      den.aspects.hosts._.server._.networking._.vlans._.apps
      den.aspects.infra._.clients._.container-loki
      den.aspects.infra._.clients._.container-prometheus
      den.aspects.infra._.clients._.container-tempo
    ];

    nixos =
      { config, ... }:
      let
        cfg = config.containerConfigs.thelounge;
      in
      {
        systemd.services."container@thelounge" = {
          after = [ "container-wave-core.target" ];
          before = [ "container-wave-apps.target" ];
        };

        containerConfigs.thelounge = {
          lldpd.enable = true;
          hostBridge = "br-apps";
          macAddress = config.infra.clients.container-thelounge.mac;
          localAddress = "${config.infra.clients.container-thelounge.ip}/${toString config.infra.vlans.apps.subnetPrefix}";
          gateway = config.infra.vlans.apps.staticAddresses.gateway;
          port = config.infra.clients.container-thelounge.ports.default;
          ldapUrl = "ldap://${config.infra.clients.container-lldap.ip}:${toString config.infra.clients.container-lldap.ports.ldap}";
          ldapBaseDN = "ou=people,dc=homelab";
          seedScript = ./_seed.py;
          # Networks enforced into users/<name>.json every boot. Edit freely.
          # nick is a sops secret (per-tracker, shared IRC identity with autobrr);
          # host/port/channels are not secret.
          seedUsers = [
            {
              name = "admin";
              networks = [
                (import ./networks/_pretome.nix)
                (import ./networks/_torrentleech.nix)
                (import ./networks/_myanonamouse.nix)
                (import ./networks/_digitalcore.nix)
                (import ./networks/_retrotoon.nix)
                (import ./networks/_libera.nix)
                (import ./networks/_rizon.nix)
                (import ./networks/_snoonet.nix)
              ];
              # Enforced into users/admin.json clientSettings every boot (merged —
              # declared keys win, other UI tweaks preserved). Theme CSS is read
              # from theme.css so it stays reproducible.
              clientSettings = {
                theme = "morning";
                searchEnabled = true;
                # Browser notifications on, notification sound off. The browser
                # still has to grant the Notification permission itself (prompted
                # on first load) — this only arms it.
                desktopNotifications = true;
                notification = false;
                userStyles = builtins.readFile ./theme.css;
              };
            }
          ];
          bindMounts = {
            "/var/lib/thelounge" = {
              hostPath = "/var/lib/appdata/thelounge";
              isReadOnly = false;
            };
            "/run/secrets/mam-username" = {
              hostPath = config.sops.secrets."mam-username".path;
              isReadOnly = true;
            };
            "/run/secrets/mam-irc-password" = {
              hostPath = config.sops.secrets."mam-irc-password".path;
              isReadOnly = true;
            };
            "/run/secrets/libera-username" = {
              hostPath = config.sops.secrets."libera-username".path;
              isReadOnly = true;
            };
            "/run/secrets/rizon-username" = {
              hostPath = config.sops.secrets."rizon-username".path;
              isReadOnly = true;
            };
            "/run/secrets/libera-nickserv-password" = {
              hostPath = config.sops.secrets."libera-nickserv-password".path;
              isReadOnly = true;
            };
            "/run/secrets/rizon-nickserv-password" = {
              hostPath = config.sops.secrets."rizon-nickserv-password".path;
              isReadOnly = true;
            };
            "/run/secrets/snoonet-username" = {
              hostPath = config.sops.secrets."snoonet-username".path;
              isReadOnly = true;
            };
            "/run/secrets/torrentleech-username" = {
              hostPath = config.sops.secrets."torrentleech-username".path;
              isReadOnly = true;
            };
            "/run/secrets/torrentleech-irc-password" = {
              hostPath = config.sops.secrets."torrentleech-irc-password".path;
              isReadOnly = true;
            };
            "/run/secrets/torrentleech-irc-key" = {
              hostPath = config.sops.secrets."torrentleech-irc-key".path;
              isReadOnly = true;
            };
            "/run/secrets/digitalcore-username" = {
              hostPath = config.sops.secrets."digitalcore-username".path;
              isReadOnly = true;
            };
            "/run/secrets/digitalcore-irc-key" = {
              hostPath = config.sops.secrets."digitalcore-irc-key".path;
              isReadOnly = true;
            };
            "/run/secrets/retrotoon-username" = {
              hostPath = config.sops.secrets."retrotoon-username".path;
              isReadOnly = true;
            };
            "/run/secrets/retrotoon-irc-password" = {
              hostPath = config.sops.secrets."retrotoon-irc-password".path;
              isReadOnly = true;
            };
          };
          nameservers = [
            config.infra.clients.container-dns.ip
            "1.1.1.1"
          ];
          logging = {
            enable = true;
            lokiAddress = "http://${config.infra.clients.container-loki.ip}:${toString config.infra.clients.container-loki.ports.default}";
            prometheusAddress = "http://${config.infra.clients.container-prometheus.ip}:${toString config.infra.clients.container-prometheus.ports.default}/api/v1/write";
            tempoAddress = "http://${config.infra.clients.container-tempo.ip}:${toString config.infra.clients.container-tempo.ports.otlp-http}/v1/traces";
          };
        };

        systemd.tmpfiles.rules = [
          "d /var/lib/appdata/thelounge 0770 ${toString cfg.uid} ${toString cfg.gid} - -"
          "Z /var/lib/appdata/thelounge 0770 ${toString cfg.uid} ${toString cfg.gid} - -"
        ];
      };
  };
}
