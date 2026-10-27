# /run is filling. Unlike a data filesystem, a full /run does not merely stop writes —
# it breaks the machine's ability to be administered: journald drops stream data, and
# logind cannot place /run/systemd/users/<uid>, so new SSH logins get no session scope
# and fail. The services already running carry on, which is what makes it easy to miss.
#
# This host reaches that state on its own. Containers run ephemeral roots, so container
# /var/log lives in host RAM charged to /run (capped in modules/containers/_base.nix);
# jellyfin's transcode cache lands there too. /run has hit 0 bytes free.
#
# Neither existing storage rule covers it: root_disk_low is mountpoint="/" only, and
# disk_fill_predicted excludes fstype=~tmpfs (predict_linear over a tmpfs that churns
# by gigabytes fires constantly). A plain static ratio is the right shape here.
#
# host_type="host": every container sidecar reports the host's own mounts, so without
# the filter one full /run fires ~50 identical alerts.
{ promRule, ... }:
{
  group = "infra";
  rules = [
    (promRule {
      uid = "run_tmpfs_low";
      title = "/run tmpfs low";
      expr = ''
        node_filesystem_avail_bytes{mountpoint="/run",host_type="host"}
          / node_filesystem_size_bytes{mountpoint="/run",host_type="host"}
      '';
      op = "lt";
      # 15% of a 23 GB /run is ~3.5 GB of headroom — enough to notice a container
      # journal or cache running away and still have room to act before ENOSPC.
      value = 0.15;
      for = "10m";
      severity = "warning";
      category = "storage";
      noDataState = "OK";
      summary = "{{ $labels.host }} /run below 15% free";
      description = "/run on {{ $labels.host }} is under 15% free. At zero, journald stops recording and logind cannot create session scopes — new SSH logins fail while everything already running looks healthy. Usual cause is a container's ephemeral root: check per-container /var/log and /var/cache sizes.";
    })
  ];
}
