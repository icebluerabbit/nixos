# mounts — custom filesystem mount points and options configuration for desktop host
_: {
  den.aspects.hosts._.desktop._.mounts = {
    aspect-name = "hosts.desktop.mounts";
    nixos = {
      # Dev workspace — dedicated 4TB NVMe. Previously exFat, which cannot store
      # symlinks, unix permissions or hardlinks; btrfs restores all three. Keyed
      # by label rather than uuid so a future reformat doesn't break this entry.
      fileSystems."/projects" = {
        device = "/dev/disk/by-label/projects";
        fsType = "btrfs";
        options = [
          "subvol=projects"
          "compress=zstd:1"
          "noatime"
          "ssd"
          "discard=async"
          "space_cache=v2"
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

      systemd.tmpfiles.rules = [
        # Library root owned by zms so Steam can create steamapps/ + compatdata/.
        "d /games 0755 zms users -"
        # Workspace root owned by zms — btrfs carries real ownership, replacing
        # the uid=/gid=/umask= mount options exFat needed to fake it.
        "d /projects 0755 zms users -"
      ];
    };
  };
}
