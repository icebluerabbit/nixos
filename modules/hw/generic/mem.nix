# mem — kernel virtual memory tuning for high-RAM systems.
#
# Sysctls only. zram lives in hw.zram, because the two are wanted independently: a
# machine with a slow or contended backing store needs these dirty limits whether or
# not it wants a compressed swap device, and the server is exactly that case (91 GB
# RAM in front of an NFS share — see the note on dirty_background_bytes below).
_: {
  den.aspects.hw._.mem = {
    aspect-name = "hw.mem";
    nixos = _: {
      boot.kernel.sysctl = {
        "vm.swappiness" = 10;
        "vm.vfs_cache_pressure" = 50;
        "vm.page-cluster" = 0;
        # Absolute dirty limits beat ratios on large RAM — 20% of 64GB = 12GB stall.
        # The stock ratios are actively dangerous in front of a network filesystem:
        # dirty pages accumulate at RAM speed and drain at link speed, so the kernel
        # reaches direct reclaim and starts evicting live anon pages to swap while a
        # writer is still filling the pool.
        "vm.dirty_background_bytes" = 67108864; # 64 MB — start background writeback
        "vm.dirty_bytes" = 1073741824; # 1 GB — hard throttle limit
      };
    };
  };
}
