# server — configuration profile aggregating all aspects and containerized services for the server host
{ den, ... }:
{
  den.aspects.hosts._.server = {
    aspect-name = "hosts.server";
    includes = [
      den.provides.hostname

      den.aspects.hosts._.server._.boot
      den.aspects.hosts._.server._.disks
      den.aspects.hosts._.server._.security
      den.aspects.hosts._.server._.networking._.links
      den.aspects.hosts._.server._.networking._.bonds
      den.aspects.hosts._.server._.networking._.routing
      den.aspects.hosts._.server._.networking._.lldpd
      den.aspects.hosts._.server._.networking._.rps
      den.aspects.hosts._.server._.networking._.bridges._.vpn1
      den.aspects.hosts._.server._.networking._.bridges._.vpn2
      den.aspects.hosts._.server._.networking._.bridges._.vpn3
      den.aspects.hosts._.server._.networking._.bridges._.vpn4
      den.aspects.hosts._.server._.networking._.bridges._.vpn5
      den.aspects.hosts._.server._.networking._.bridges._.vpn6
      den.aspects.hosts._.server._.networking._.bridges._.vpn7
      den.aspects.hosts._.server._.networking._.bridges._.vpn8
      den.aspects.hosts._.server._.networking._.bridges._.vpn9
      den.aspects.hosts._.server._.networking._.bridges._.vpn10
      den.aspects.hosts._.server._.networking._.vlans._.apps
      den.aspects.hosts._.server._.networking._.vlans._.dmz
      den.aspects.hosts._.server._.networking._.vlans._.download
      den.aspects.hosts._.server._.networking._.vlans._.game
      den.aspects.hosts._.server._.networking._.vlans._.guest
      den.aspects.hosts._.server._.networking._.vlans._.iot
      den.aspects.hosts._.server._.networking._.vlans._.media
      den.aspects.hosts._.server._.networking._.vlans._.services
      den.aspects.hosts._.server._.networking._.vlans._.storage
      den.aspects.hosts._.server._.networking._.vlans._.trust
      den.aspects.hosts._.server._.networking._.vlans._.work
      den.aspects.hosts._.server._.containers._.caddy
      den.aspects.hosts._.server._.containers._.dnsmasq
      den.aspects.hosts._.server._.containers._.homepage
      den.aspects.hosts._.server._.containers._.gitea
      den.aspects.hosts._.server._.containers._.esphome
      den.aspects.hosts._.server._.containers._.home-assistant
      den.aspects.hosts._.server._.containers._.zwave
      den.aspects.hosts._.server._.containers._.mosquitto
      den.aspects.hosts._.server._.containers._.zigbee2mqtt
      den.aspects.hosts._.server._.containers._.pi-hole
      den.aspects.hosts._.server._.containers._.kiwix
      den.aspects.hosts._.server._.containers._.postgresql
      den.aspects.hosts._.server._.containers._.vpn-gw
      den.aspects.hosts._.server._.containers._.qbittorrent
      den.aspects.hosts._.server._.containers._.slskd
      den.aspects.hosts._.server._.containers._.soularr
      den.aspects.hosts._.server._.containers._.qui
      den.aspects.hosts._.server._.containers._.pgadmin
      den.aspects.hosts._.server._.containers._.radarr
      den.aspects.hosts._.server._.containers._.jellyfin
      den.aspects.hosts._.server._.containers._.lidarr
      den.aspects.hosts._.server._.containers._.beets
      den.aspects.hosts._.server._.containers._.readarr
      den.aspects.hosts._.server._.containers._.calibre-web
      den.aspects.hosts._.server._.containers._.bindery
      den.aspects.hosts._.server._.containers._.komga
      den.aspects.hosts._.server._.containers._.audiobookshelf
      den.aspects.hosts._.server._.containers._.sonarr
      den.aspects.hosts._.server._.containers._.bazarr
      den.aspects.hosts._.server._.containers._.prowlarr
      den.aspects.hosts._.server._.containers._.managarr
      den.aspects.hosts._.server._.containers._.flaresolverr
      den.aspects.hosts._.server._.containers._.cross-seed
      den.aspects.hosts._.server._.containers._.seasonpackarr
      den.aspects.hosts._.server._.containers._.autobrr
      den.aspects.hosts._.server._.containers._.mousetrap
      den.aspects.hosts._.server._.containers._.unpackerr
      den.aspects.hosts._.server._.containers._.youtarr
      den.aspects.hosts._.server._.containers._.grafana
      den.aspects.hosts._.server._.containers._.prometheus
      den.aspects.hosts._.server._.containers._.loki
      den.aspects.hosts._.server._.containers._.tempo
      den.aspects.hosts._.server._.containers._.unifi-exporter
      den.aspects.hosts._.server._.containers._.opencode
      den.aspects.hosts._.server._.containers._.openchamber
      den.aspects.hosts._.server._.containers._.code-server
      den.aspects.hosts._.server._.containers._.authelia
      den.aspects.hosts._.server._.containers._.tuwunel
      den.aspects.hosts._.server._.containers._.hermes
      den.aspects.hosts._.server._.containers._.lldap
      den.aspects.hosts._.server._.containers._.cinny
      den.aspects.hosts._.server._.containers._.thelounge
      den.aspects.hosts._.server._.containers
      den.aspects.hosts._.server._.platform
      den.aspects.hosts._.server._.time
      den.aspects.hosts._.server._.services._.ssh
      den.aspects.hosts._.server._.backups
      den.aspects.services._.btrfs-scrub
      den.aspects.services._.btrfs-balance
      den.aspects.services._.nixos-notify

      den.aspects.hw._.asrockrack-b650d4u3
      den.aspects.hw._.amd-epyc-4545p
      den.aspects.hw._.crucial-t705
      den.aspects.hw._.wd-sn580
      den.aspects.hw._.intel-arc-b570
      den.aspects.hw._.corsair-nautilus-360
      # Sysctls only — hw.zram is deliberately not included here. This host's swap
      # pressure comes from writeback against the NFS share, which absolute dirty
      # limits fix and a compressed swap device only papers over.
      den.aspects.hw._.mem

      den.aspects.groups._.nixos-config
      den.aspects.groups._.video
      den.aspects.groups._.render
      den.aspects.groups._.workspace
      den.aspects.security._.sops
      den.aspects.security._.certs
      den.aspects.logging._.vector
      den.aspects.infra._.homelab
      den.aspects.style._.stylix
      den.aspects.cli._.neovim
    ];

    nixos =
      { config, ... }:
      {
        hardware.coolercontrol.settings = {
          ipv4_address = config.infra.clients.server.ip;
        };
        # co-write the shared /var/lib/workspace tree with the code-server IDE container
        users.users.zms.extraGroups = [ "workspace" ];
      };

    homeManager = _: {
      programs.mcp.servers.filesystem.args = [
        "/etc/nixos"
        "/var/log"
        "/var/lib/appdata"
      ];
    };
  };
}
