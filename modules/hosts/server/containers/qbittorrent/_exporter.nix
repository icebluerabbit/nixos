# Vector logging extras for qbittorrent: the martabal exporter scrape and the per-peer
# choke/interest aggregator (qbt-peer-exporter → exec → log_to_metric). VPN
# exit-IP/forwarded-port metrics now come from the vpn-gw broker's /export, not a
# qbt journal line.
{
  exporterPort,
  webuiPort,
  pkgs,
  capN ? 200,
  ...
}:
{
  prometheusInputs = [
    "qbt_exporter"
    "qbt_peer_metrics"
  ];
  extraSettings = {
    sources.qbt_exporter = {
      type = "prometheus_scrape";
      endpoints = [
        "http://127.0.0.1:${toString exporterPort}/metrics"
      ];
      scrape_interval_secs = 30;
    };
    # Per-peer flags carry the choke/interest state the WebAPI global surface omits.
    # No global peer list exists, so qbt-peer-exporter loops /sync/torrentPeers over ACTIVE
    # torrents (top-upspeed capped at capN, wall-clock budgeted) and prints one JSON
    # line of aggregates. Cost is bound by active-torrent count, not library size.
    sources.qbt_peer_exec = {
      type = "exec";
      mode = "scheduled";
      scheduled.exec_interval_secs = 60;
      command = [
        "${pkgs.homelab.qbt-scripts}/bin/qbt-peer-exporter"
        "http://127.0.0.1:${toString webuiPort}"
        (toString capN)
      ];
    };
    transforms = {
      # qbt-peer-exporter stdout (one JSON line) → event fields → gauges.
      qbt_peer_parse = {
        type = "remap";
        inputs = [ "qbt_peer_exec" ];
        source = ". = object!(parse_json!(string!(.message)))";
      };
      qbt_peer_metrics = {
        type = "log_to_metric";
        inputs = [ "qbt_peer_parse" ];
        metrics = [
          {
            type = "gauge";
            field = "peers_total";
            name = "qbt_peers_total";
          }
          {
            type = "gauge";
            field = "uploading";
            name = "qbt_peers_uploading";
          }
          {
            type = "gauge";
            field = "interested_choked";
            name = "qbt_peers_interested_choked";
          }
          {
            type = "gauge";
            field = "downloading";
            name = "qbt_peers_downloading";
          }
          {
            type = "gauge";
            field = "optimistic";
            name = "qbt_peers_optimistic";
          }
          {
            type = "gauge";
            field = "snubbed";
            name = "qbt_peers_snubbed";
          }
          {
            type = "gauge";
            field = "upload_active";
            name = "qbt_peers_upload_active";
          }
          {
            type = "gauge";
            field = "incoming";
            name = "qbt_peers_incoming";
          }
          {
            type = "gauge";
            field = "outgoing";
            name = "qbt_peers_outgoing";
          }
          {
            type = "gauge";
            field = "peer_up_max";
            name = "qbt_peer_up_max";
          }
          {
            type = "gauge";
            field = "conn_bt";
            name = "qbt_peers_conn_bt";
          }
          {
            type = "gauge";
            field = "conn_utp";
            name = "qbt_peers_conn_utp";
          }
          {
            type = "gauge";
            field = "conn_web";
            name = "qbt_peers_conn_web";
          }
          {
            type = "gauge";
            field = "up_bytes_tcp";
            name = "qbt_up_speed_tcp";
          }
          {
            type = "gauge";
            field = "up_bytes_utp";
            name = "qbt_up_speed_utp";
          }
          {
            type = "gauge";
            field = "up_bytes_web";
            name = "qbt_up_speed_web";
          }
          {
            type = "gauge";
            field = "torrents_active";
            name = "qbt_scrape_torrents_active";
          }
          {
            type = "gauge";
            field = "torrents_scraped";
            name = "qbt_scrape_torrents_scraped";
          }
          {
            type = "gauge";
            field = "capped";
            name = "qbt_scrape_capped";
          }
          {
            type = "gauge";
            field = "scrape_ok";
            name = "qbt_scrape_ok";
          }
        ];
      };
    };
  };
}
