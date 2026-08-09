/*
  btrfs-scrub — periodic Btrfs filesystem integrity check; reads all data/metadata and verifies checksums
  https://btrfs.readthedocs.io/en/latest/btrfs-scrub.html
*/
_: {
  den.aspects.services._.btrfs-scrub = {
    aspect-name = "services.btrfs-scrub";
    nixos = {
      services.btrfs.autoScrub = {
        enable = true;
        interval = "monthly";
        fileSystems = [ "/" ];
      };
    };
  };
}
