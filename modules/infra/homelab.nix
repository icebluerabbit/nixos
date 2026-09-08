# home network
{ den, ... }:
{
  den.aspects.infra._.homelab = {
    aspect-name = "infra.homelab";
    includes = [
      den.aspects.infra._.options
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.providers._.gitea

      den.aspects.infra._.gitea._.repos._.nixos

      den.aspects.infra._.devices._.usw-flex-2-5g-8
      den.aspects.infra._.devices._.usw-pro-hd-24-poe
      den.aspects.infra._.devices._.u7-pro-xg
      den.aspects.infra._.devices._.dream-machine-pro-max

      den.aspects.infra._.dyndns._.cloudflare

      den.aspects.infra._.cloudflare._.zones._.primary
      den.aspects.infra._.cloudflare._.dns._.home

      den.aspects.infra._.wans._.verizon-fios-ont

      den.aspects.infra._.vpns._.one-click-vpn

      den.aspects.infra._.ports._.ap-trunk
      den.aspects.infra._.ports._.gateway-trunk
      den.aspects.infra._.ports._.living-room-trunk
      den.aspects.infra._.ports._.server-trunk
      den.aspects.infra._.ports._.work-macbookpro-work-1
      den.aspects.infra._.ports._.iot-hue-hub
      den.aspects.infra._.ports._.game-playstation5
      den.aspects.infra._.ports._.storage-unas-pro
      den.aspects.infra._.ports._.mgmt-server
      den.aspects.infra._.ports._.mgmt-server-ipmi
      den.aspects.infra._.ports._.trust-desktop

      den.aspects.infra._.networks._.default
      den.aspects.infra._.networks._.mgmt
      den.aspects.infra._.networks._.trust
      den.aspects.infra._.networks._.iot
      den.aspects.infra._.networks._.guest
      den.aspects.infra._.networks._.storage
      den.aspects.infra._.networks._.work
      den.aspects.infra._.networks._.game
      den.aspects.infra._.networks._.dmz
      den.aspects.infra._.networks._.apps
      den.aspects.infra._.networks._.media
      den.aspects.infra._.networks._.download
      den.aspects.infra._.networks._.services

      den.aspects.infra._.clients._.broadlink-remote
      den.aspects.infra._.clients._.presence-sensor-fp2-db2e
      den.aspects.infra._.clients._.presence-sensor-fp2-96e8
      den.aspects.infra._.clients._.reterminal-e1003
      den.aspects.infra._.clients._.container-vpn-gw
      den.aspects.infra._.clients._.container-qbittorrent
      den.aspects.infra._.clients._.container-slskd
      den.aspects.infra._.clients._.desktop
      den.aspects.infra._.clients._.hue-hub
      den.aspects.infra._.clients._.server
      den.aspects.infra._.clients._.server-trunk
      den.aspects.infra._.clients._.server-ipmi
      den.aspects.infra._.clients._.server-vlan-trust
      den.aspects.infra._.clients._.server-vlan-iot
      den.aspects.infra._.clients._.server-vlan-guest
      den.aspects.infra._.clients._.server-vlan-storage
      den.aspects.infra._.clients._.server-vlan-work
      den.aspects.infra._.clients._.server-vlan-game
      den.aspects.infra._.clients._.server-vlan-dmz
      den.aspects.infra._.clients._.server-vlan-apps
      den.aspects.infra._.clients._.server-vlan-media
      den.aspects.infra._.clients._.server-vlan-download
      den.aspects.infra._.clients._.server-bridge-download
      den.aspects.infra._.clients._.server-bridge-trust
      den.aspects.infra._.clients._.server-bridge-iot
      den.aspects.infra._.clients._.server-bridge-guest
      den.aspects.infra._.clients._.server-bridge-storage
      den.aspects.infra._.clients._.server-bridge-work
      den.aspects.infra._.clients._.server-bridge-game
      den.aspects.infra._.clients._.server-bridge-dmz
      den.aspects.infra._.clients._.server-bridge-apps
      den.aspects.infra._.clients._.server-bridge-media
      den.aspects.infra._.clients._.server-bridge-services
      den.aspects.infra._.clients._.container-qui
      den.aspects.infra._.clients._.container-postgres
      den.aspects.infra._.clients._.container-pgadmin
      den.aspects.infra._.clients._.container-radarr
      den.aspects.infra._.clients._.container-lidarr
      den.aspects.infra._.clients._.container-beets
      den.aspects.infra._.clients._.container-readarr
      den.aspects.infra._.clients._.container-calibre-web
      den.aspects.infra._.clients._.container-bindery
      den.aspects.infra._.clients._.container-komga
      den.aspects.infra._.clients._.container-audiobookshelf
      den.aspects.infra._.clients._.container-sonarr
      den.aspects.infra._.clients._.container-bazarr
      den.aspects.infra._.clients._.container-prowlarr
      den.aspects.infra._.clients._.container-stash
      den.aspects.infra._.clients._.container-flaresolverr
      den.aspects.infra._.clients._.container-cross-seed
      den.aspects.infra._.clients._.container-cleanuparr
      den.aspects.infra._.clients._.container-seasonpackarr
      den.aspects.infra._.clients._.container-autobrr
      den.aspects.infra._.clients._.container-mousetrap
      den.aspects.infra._.clients._.container-unpackerr
      den.aspects.infra._.clients._.container-soularr
      den.aspects.infra._.clients._.container-youtarr
      den.aspects.infra._.clients._.container-jellyfin
      den.aspects.infra._.clients._.container-kiwix
      den.aspects.infra._.clients._.container-gitea
      den.aspects.infra._.clients._.container-home-assistant
      den.aspects.infra._.clients._.container-zwave
      den.aspects.infra._.clients._.container-mosquitto
      den.aspects.infra._.clients._.container-zigbee2mqtt
      den.aspects.infra._.clients._.container-esphome
      den.aspects.infra._.clients._.container-caddy
      den.aspects.infra._.clients._.container-dns
      den.aspects.infra._.clients._.container-homepage
      den.aspects.infra._.clients._.container-pi-hole
      den.aspects.infra._.clients._.container-opencode
      den.aspects.infra._.clients._.container-openchamber
      den.aspects.infra._.clients._.container-code-server
      den.aspects.infra._.clients._.container-loki
      den.aspects.infra._.clients._.container-grafana
      den.aspects.infra._.clients._.container-prometheus
      den.aspects.infra._.clients._.container-tempo
      den.aspects.infra._.clients._.container-authelia
      den.aspects.infra._.clients._.container-tuwunel
      den.aspects.infra._.clients._.container-hermes
      den.aspects.infra._.clients._.container-lldap
      den.aspects.infra._.clients._.container-cinny
      den.aspects.infra._.clients._.container-thelounge
      den.aspects.infra._.clients._.container-unifi-exporter
      den.aspects.infra._.clients._.server-vlan-services
      den.aspects.infra._.clients._.unas-pro
      den.aspects.infra._.clients._.macbookpro-work-1
      den.aspects.infra._.clients._.macbookpro-work-2
      den.aspects.infra._.clients._.macbookpro-personal
      den.aspects.infra._.clients._.iphone
      den.aspects.infra._.clients._.ipad
      den.aspects.infra._.clients._.google-home
      den.aspects.infra._.clients._.playstation5

      den.aspects.infra._.usergroups._.default

      den.aspects.infra._.settings._.default

      den.aspects.infra._.wlans._.herp
      den.aspects.infra._.wlans._.derp
      den.aspects.infra._.wlans._.werp
    ];
  };
}
