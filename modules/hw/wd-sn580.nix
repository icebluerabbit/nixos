/*
  wd-sn580 — WD Blue SN580 NVMe SSD (DRAM-less, /var/cache/fscache backing store)

  nvme.max_host_mem_size_mb is a module param on nvme.ko and applies host-wide to
  every NVMe controller, not just this drive. There is no per-device HMB control.
*/
_: {
  den.aspects.hw._.wd-sn580 = {
    aspect-name = "hw.wd-sn580";
    nixos = {
      boot.kernelParams = [ "nvme.max_host_mem_size_mb=0" ];
    };
  };
}
