# desktop — configuration profile aggregating all aspects and hardware profiles for the desktop workstation host
{ den, ... }:
{
  den.aspects.hosts._.desktop = {
    aspect-name = "hosts.desktop";
    includes = [
      den.provides.hostname
      den.aspects.hosts._.desktop._.backups
      den.aspects.hosts._.desktop._.boot
      den.aspects.hosts._.desktop._.disks
      den.aspects.hosts._.desktop._.flake-path
      den.aspects.hosts._.desktop._.impermanence
      den.aspects.hosts._.desktop._.mounts
      den.aspects.hosts._.desktop._.networking
      den.aspects.hosts._.desktop._.platform
      den.aspects.hosts._.desktop._.time
      den.aspects.hw._.amd-9800-x3d
      den.aspects.hw._.logitech-mouse
      den.aspects.hw._.samsung-ssd
      den.aspects.hw._.nvidia-rtx-4090
      den.aspects.hw._.sandisk-usb
      den.aspects.hw._.lg-oled48cx
      den.aspects.hw._.steelseries-arctis-nova-pro
      den.aspects.hw._.corsair-dominator-titanium
      den.aspects.hw._.corsair-mm700
      den.aspects.hw._.lian-li-strimer-v3
      den.aspects.hw._.nuphy-kick75
      den.aspects.hw._.nzxt-kraken-elite-v2
      den.aspects.hw._.asus-rog-strix-x870e-a
      den.aspects.hw._.nzxt-rgb-fan-controller
      # den.aspects.hw._.razer-deathadder-v3-pro
      den.aspects.services._.ssh
      den.aspects.services._.btrfs-scrub
      den.aspects.services._.nixos-notify
      den.aspects.security._.sops
      den.aspects.security._.certs
      den.aspects.infra._.homelab
      den.aspects.services._.upower
      den.aspects.services._.logind
      den.aspects.services._.gnome-keyring
      den.aspects.groups._.nixos-config
      den.aspects.logging._.vector
      den.aspects.cli._.neovim
    ];

    homeManager = _: {
      programs.mcp.servers.filesystem.args = [
        "/etc/nixos"
        "/projects"
      ];
    };
  };
}
