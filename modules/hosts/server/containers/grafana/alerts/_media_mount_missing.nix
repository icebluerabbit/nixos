# The NFS mount vanished from the server entirely. Distinct from "the share is full"
# and much worse: every arr, qbittorrent and Jellyfin lose their library at once, and
# the arrs will happily start re-downloading a library they think is gone.
#
# absent() on the mount's own series — if node_exporter can't see the mountpoint, it
# isn't mounted.
{ promRule, ... }:
{
  group = "infra";
  rules = [
    (promRule {
      uid = "media_mount_missing";
      title = "Media mount missing";
      expr = ''absent(node_filesystem_size_bytes{host="server",mountpoint="/mnt/media"})'';
      op = "gt";
      value = 0;
      for = "5m";
      severity = "critical";
      category = "storage";
      noDataState = "OK";
      summary = "/mnt/media is not mounted on server";
      description = "The NFS media share is no longer mounted on server. Every media container has lost its library — expect the arrs to treat the catalogue as missing.";
    })
  ];
}
