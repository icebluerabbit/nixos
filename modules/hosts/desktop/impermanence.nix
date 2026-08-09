# impermanence — persistent file and directory paths definition for the ephemeral root desktop workstation
{ den, ... }:
{
  den.aspects.hosts._.desktop._.impermanence = {
    aspect-name = "hosts.desktop.impermanence";
    includes = [ den.aspects.boot._.impermanence ];

    environment.persistence."/persist" = {
      enable = true;
      hideMounts = true;
      directories = [
        "/var/lib/coolercontrol"
        "/var/lib/systemd/coredump"
        "/var/lib/sops-nix"
        "/var/lib/nixos"
        "/var/lib/OpenRGB"
      ];
      files = [
        "/etc/machine-id"
      ];
    };
  };
}
