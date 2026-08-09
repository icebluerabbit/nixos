# qBittorrent's own free-space view of the download target. Redundant with the media
# share rule only when they point at the same volume — this one is what qBittorrent
# will actually refuse to write to.
{ promRule, ... }:
{
  group = "media";
  rules = [
    (promRule {
      uid = "qbt_disk_low";
      title = "qBittorrent disk low";
      expr = "qbittorrent_global_free_space_on_disk_bytes";
      op = "lt";
      value = 200000000000;
      for = "30m";
      severity = "warning";
      category = "qbittorrent";
      noDataState = "OK";
      summary = "qBittorrent has under 200G free";
      description = "The volume qBittorrent writes to has less than 200G free — torrents will start erroring on write.";
    })
  ];
}
