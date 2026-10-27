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
        # restic's index/metadata cache — without this the nightly /projects
        # backup refetches the whole repo index from the NAS after every boot.
        "/var/cache/restic"
      ];
      files = [
        "/etc/machine-id"
      ];
    };
  };
}
