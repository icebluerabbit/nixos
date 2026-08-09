# tuwunel — host configuration override for tuwunel Matrix homeserver container
{ den, ... }:
{
  den.aspects.hosts._.server._.containers._.tuwunel = {
    aspect-name = "hosts.server.containers.tuwunel";
    includes = [
      den.aspects.containers._.tuwunel
      den.aspects.infra._.clients._.container-tuwunel
      den.aspects.infra._.clients._.container-dns
      den.aspects.infra._.networks._.services
      den.aspects.hosts._.server._.security
      den.aspects.hosts._.server._.networking._.vlans._.services
      den.aspects.infra._.clients._.container-loki
      den.aspects.infra._.clients._.container-prometheus
      den.aspects.infra._.clients._.container-tempo
    ];

    nixos =
      { config, ... }:
      let
        cfg = config.containerConfigs.tuwunel;
      in
      {
        systemd.services."container@tuwunel" = {
          after = [ "container-wave-foundation.target" ];
          before = [ "container-wave-core.target" ];
        };

        containerConfigs.tuwunel = {
          lldpd.enable = true;
          hostBridge = "br-services";
          macAddress = config.infra.clients.container-tuwunel.mac;
          localAddress = "${config.infra.clients.container-tuwunel.ip}/${toString config.infra.vlans.services.subnetPrefix}";
          gateway = config.infra.vlans.services.staticAddresses.gateway;
          port = config.infra.clients.container-tuwunel.ports.default;
          initUsers = [
            {
              username = "admin";
              admin = true;
              avatar = "user-admin";
            }
            {
              username = "ha-bot";
              avatar = "user-info";
            }
            {
              username = "hermes";
              avatar = "assistant";
            }
          ];
          initRooms =
            let
              # HA (ha-bot) is the current sender identity; join it to every
              # content room so matrix.send_message can post there. agentRoom
              # additionally joins the hermes agent, which reads as well as writes.
              # Last arg is the Papirus app-icon name for the room avatar.
              mkRoom = members: alias: name: parent: icon: {
                inherit
                  alias
                  name
                  parent
                  members
                  icon
                  ;
              };
              room = mkRoom [ "ha-bot" ];
              agentRoom = mkRoom [
                "ha-bot"
                "hermes"
              ];
              space = alias: name: parent: icon: {
                inherit
                  alias
                  name
                  parent
                  icon
                  ;
                space = true;
              };
            in
            [
              # Umbrella + domain spaces.
              (space "homelab" "Homelab" "" "applications-system")
              (space "media" "Media" "homelab" "applications-multimedia")
              (space "home" "Home" "homelab" "preferences-desktop-personal")
              (space "network" "Network" "homelab" "preferences-system-network")
              (space "infra" "Infra" "homelab" "applications-development")

              # Severity fan-out (top-level under the umbrella).
              (room "alerts" "Alerts" "homelab" "bell")

              # Conversational agent (top-level; talks back, unlike the fan-out rooms).
              (agentRoom "hermes" "Hermes" "homelab" "assistant")

              # Media.
              (room "radarr" "Radarr" "media" "multimedia-video-player")
              (room "sonarr" "Sonarr" "media" "kaffeine")
              (room "lidarr" "Lidarr" "media" "rhythmbox")
              (room "prowlarr" "Prowlarr" "media" "kfind")
              (room "bazarr" "Bazarr" "media" "subtitleeditor")
              (room "autobrr" "autobrr" "media" "hexchat")
              (room "myanonamouse" "MyAnonaMouse" "media" "calibre")
              (room "qbittorrent" "qBittorrent" "media" "qbittorrent")
              (room "cross-seed" "cross-seed" "media" "transmission")
              (room "soulseek" "Soulseek" "media" "soulseek")
              (room "jellyfin" "Jellyfin" "media" "jellyfin")
              (room "youtarr" "Youtarr" "media" "youtube")
              (room "lazylibrarian" "LazyLibrarian" "media" "calibre")

              # Home.
              (room "home-assistant" "Home Assistant" "home" "preferences-system")
              (room "automations" "Automations" "home" "node-red-icon")

              # Network.
              (room "unifi" "UniFi" "network" "knemo")
              (room "vpn-gw" "VPN Gateway" "network" "openvpn")
              (room "dns" "DNS" "network" "applications-internet")
              (room "clients" "Clients" "network" "org.gnome.Boxes")

              # Infra.
              (room "nixos" "NixOS" "infra" "nix-snowflake")
              (room "ci" "CI" "infra" "git")
              (room "backups" "Backups" "infra" "restic")

              # Mail. One room per concern; #inbox is the broad relay, narrower
              # imap entries (OTP, receipts) get their own rooms alongside it.
              (space "mail" "Mail" "homelab" "internet-mail")
              (room "inbox" "Inbox" "mail" "email")

              # GitHub. One room per event kind, repo name prefixed in the
              # message — the watched repo list lives in the HA aspect.
              (space "github" "GitHub" "infra" "github")
              (room "releases" "Releases" "github" "system-software-install")
              (room "pull-requests" "Pull Requests" "github" "git")
              (room "issues" "Issues" "github" "bug-buddy")
              (room "commits" "Commits" "github" "gitg")
              (room "stars" "Stars" "github" "github")
            ];
          envFile = config.sops.templates."tuwunel-env".path;
          # Shared secret for the init script's /_synapse/admin/v1/register calls (replaces
          # the old --execute user bootstrap). LAN-only homeserver, no federation.
          registrationSharedSecret = "b7e4c1a9f230d85e6c4718fa93be05d271c8a4f60e9b32d7581ac64ef0927d3b";
          bindMounts."/run/secrets/tuwunel-env" = {
            hostPath = config.sops.templates."tuwunel-env".path;
            isReadOnly = true;
          };
          bindMounts."/var/lib/tuwunel" = {
            hostPath = "/var/lib/appdata/tuwunel";
            isReadOnly = false;
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
          "d /var/lib/appdata/tuwunel 0700 ${toString cfg.uid} ${toString cfg.gid} - -"
          "Z /var/lib/appdata/tuwunel 0700 ${toString cfg.uid} ${toString cfg.gid} - -"
        ];
      };
  };
}
