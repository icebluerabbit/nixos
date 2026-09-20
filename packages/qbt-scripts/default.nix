/*
  qbt-scripts — qBittorrent WebAPI helpers that run alongside the container.

    qbt-reannounce     re-announce not-yet-started torrents whose trackers
                       have all failed
    qbt-watch          import .torrent files from watch/<category>/
    qbt-peer-exporter  per-peer choke/interest aggregation, one JSON line on
                       stdout for Vector's exec source to turn into gauges

  Each takes its target WebAPI URL (and any tuning) on argv, so nothing here
  knows about a particular host.
*/
{
  writers,
  symlinkJoin,
}:
let
  script = name: src: writers.writePython3Bin name { } (builtins.readFile src);
in
symlinkJoin {
  name = "qbt-scripts";
  paths = [
    (script "qbt-reannounce" ./reannounce.py)
    (script "qbt-watch" ./watch.py)
    (script "qbt-peer-exporter" ./exporter.py)
  ];
  meta.description = "qBittorrent WebAPI helper scripts";
}
