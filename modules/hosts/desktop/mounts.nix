# mounts — custom filesystem mount points and options configuration for desktop host
_: {
  den.aspects.hosts._.desktop._.mounts = {
    aspect-name = "hosts.desktop.mounts";
    nixos = {
      fileSystems."/projects" = {
        device = "/dev/disk/by-uuid/805F-4BA7";
        fsType = "exfat";
        options = [
          "uid=1000"
          "gid=1000"
          "umask=0022"
          "nofail"
        ];
      };

      # Steam/Proton games library — whole-disk btrfs on the 990 PRO. nodatacow
      # avoids CoW fragmentation from game updates (and disables compression +
      # checksum, both pointless on already-compressed re-downloadable assets).
      fileSystems."/games" = {
        device = "/dev/disk/by-label/games";
        fsType = "btrfs";
        options = [
          "subvol=games"
          "nodatacow"
          "noatime"
          "ssd"
          "discard=async"
          "space_cache=v2"
          "commit=120"
          "nofail"
        ];
      };

      # Library root owned by zms so Steam can create steamapps/ + compatdata/.
      systemd.tmpfiles.rules = [ "d /games 0755 zms users -" ];
    };
  };
}
