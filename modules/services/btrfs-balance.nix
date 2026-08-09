/*
  btrfs-balance — periodic targeted chunk reclaim.

  Btrfs allocates space in two steps: it carves the raw device into block-group *chunks*,
  then fills chunks with data. Deleting files frees space *inside* a chunk but does not
  return the chunk to unallocated — so a churny workload (qbt temp files, DB rewrites)
  slowly converts unallocated space into mostly-empty-but-still-allocated data chunks.
  Left long enough the device runs out of *unallocated* space while df still shows free
  room, and the next metadata chunk allocation fails with ENOSPC on a "half-empty" disk.

  A filtered balance rewrites only the block groups below the -dusage/-musage fullness
  cutoff, packing their live data together and returning the emptied chunks to
  unallocated. Low percentages keep it cheap and targeted (no full-pool rewrite). Kernel
  auto-removes fully-empty chunks on its own; this handles the partially-empty ones it
  won't touch.
*/
_: {
  den.aspects.services._.btrfs-balance = {
    aspect-name = "services.btrfs-balance";
    nixos =
      { pkgs, ... }:
      {
        systemd.services.btrfs-balance = {
          description = "Btrfs periodic filtered balance (reclaim partially-empty chunks)";
          serviceConfig = {
            Type = "oneshot";
            ExecStart = "${pkgs.btrfs-progs}/bin/btrfs balance start -dusage=50 -musage=30 /";
            # Background maintenance: never contend with live services for CPU or IO.
            Nice = 19;
            IOSchedulingClass = "idle";
          };
        };
        systemd.timers.btrfs-balance = {
          wantedBy = [ "timers.target" ];
          timerConfig = {
            OnCalendar = "monthly";
            Persistent = true;
            RandomizedDelaySec = "1h";
          };
        };
      };
  };
}
