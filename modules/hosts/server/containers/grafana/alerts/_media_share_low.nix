# The NFS media share (UNAS-Pro) running out. Slow-moving by nature, so a long `for`
# and a warning severity: this is a "plan a cleanup" alert, not a page.
{ promRule, ... }:
{
  group = "infra";
  rules = [
    (promRule {
      uid = "media_share_low";
      title = "Media share low";
      expr = ''node_filesystem_avail_bytes{host="server",mountpoint="/mnt/media"}'';
      op = "lt";
      value = 500000000000;
      for = "30m";
      severity = "warning";
      category = "storage";
      noDataState = "OK";
      summary = "Media share below 500G free";
      description = "/mnt/media on server has less than 500G free. Downloads will start failing once the arrs can't import.";
    })
  ];
}
