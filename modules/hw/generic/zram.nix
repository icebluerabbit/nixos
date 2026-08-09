# zram — compressed swap in RAM.
#
# Split out of hw.mem so the dirty-page sysctls can be adopted without also adopting a
# swap device. A host whose swap pressure comes from a writeback stall wants the former
# and not the latter: adding zram there buys compressed room for pages that should never
# have been evicted, and spends CPU doing it.
_: {
  den.aspects.hw._.zram = {
    aspect-name = "hw.zram";
    nixos = _: {
      zramSwap = {
        enable = true;
        algorithm = "zstd";
        memoryPercent = 25;
      };
    };
  };
}
