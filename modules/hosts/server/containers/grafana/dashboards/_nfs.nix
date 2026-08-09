let
  prom = {
    type = "prometheus";
    uid = "Prometheus";
  };
  mount = ''{host="$host",export=~"$export"}'';
in
{
  folder = "Infrastructure";
  title = "NFS";
  uid = "nfs";
  tags = [
    "infra"
    "nfs"
    "storage"
  ];
  editable = true;
  schemaVersion = 38;
  refresh = "30s";
  time = {
    from = "now-6h";
    to = "now";
  };
  templating.list = [
    {
      name = "host";
      type = "query";
      datasource = prom;
      query = "label_values(node_mountstats_nfs_total_read_bytes_total{host_type=\"host\"}, host)";
      refresh = 1;
      includeAll = false;
      multi = false;
      current = {
        text = "server";
        value = "server";
      };
    }
    {
      name = "export";
      type = "query";
      datasource = prom;
      query = "label_values(node_mountstats_nfs_total_read_bytes_total{host=\"$host\"}, export)";
      refresh = 1;
      includeAll = true;
      multi = true;
      current = { };
    }
  ];
  panels = [

    # ── KPIs ──────────────────────────────────────────────────────────────
    {
      title = "Read Throughput";
      type = "stat";
      id = 1;
      gridPos = {
        h = 4;
        w = 6;
        x = 0;
        y = 0;
      };
      targets = [
        {
          datasource = prom;
          expr = "sum(rate(node_mountstats_nfs_total_read_bytes_total${mount}[5m]))";
          refId = "A";
        }
      ];
      options = {
        reduceOptions.calcs = [ "lastNotNull" ];
        colorMode = "background";
      };
      fieldConfig.defaults.unit = "Bps";
    }
    {
      title = "Write Throughput";
      type = "stat";
      id = 2;
      gridPos = {
        h = 4;
        w = 6;
        x = 6;
        y = 0;
      };
      targets = [
        {
          datasource = prom;
          expr = "sum(rate(node_mountstats_nfs_total_write_bytes_total${mount}[5m]))";
          refId = "A";
        }
      ];
      options = {
        reduceOptions.calcs = [ "lastNotNull" ];
        colorMode = "background";
      };
      fieldConfig.defaults.unit = "Bps";
    }
    {
      title = "READ ops/s";
      type = "stat";
      id = 3;
      gridPos = {
        h = 4;
        w = 6;
        x = 12;
        y = 0;
      };
      targets = [
        {
          datasource = prom;
          expr = ''sum(rate(node_mountstats_nfs_operations_requests_total{host="$host",export=~"$export",operation="READ"}[5m]))'';
          refId = "A";
        }
      ];
      options = {
        reduceOptions.calcs = [ "lastNotNull" ];
        colorMode = "background";
      };
      fieldConfig.defaults.unit = "ops";
    }
    {
      title = "RPC Retransmissions";
      type = "stat";
      id = 4;
      gridPos = {
        h = 4;
        w = 6;
        x = 18;
        y = 0;
      };
      targets = [
        {
          datasource = prom;
          expr = ''sum(rate(node_nfs_rpc_retransmissions_total{host="$host"}[5m]))'';
          refId = "A";
        }
      ];
      options = {
        reduceOptions.calcs = [ "lastNotNull" ];
        colorMode = "background";
      };
      fieldConfig.defaults = {
        unit = "ops";
        thresholds = {
          mode = "absolute";
          steps = [
            {
              color = "green";
              value = null;
            }
            {
              color = "red";
              value = 1;
            }
          ];
        };
      };
    }

    # ── Throughput ────────────────────────────────────────────────────────
    {
      title = "Throughput by Mount";
      type = "timeseries";
      id = 10;
      gridPos = {
        h = 8;
        w = 12;
        x = 0;
        y = 4;
      };
      targets = [
        {
          datasource = prom;
          expr = "rate(node_mountstats_nfs_total_read_bytes_total${mount}[5m])";
          legendFormat = "read {{export}}";
          refId = "A";
        }
        {
          datasource = prom;
          expr = "rate(node_mountstats_nfs_total_write_bytes_total${mount}[5m])";
          legendFormat = "write {{export}}";
          refId = "B";
        }
      ];
      fieldConfig.defaults.unit = "Bps";
    }
    {
      title = "Bytes Read vs Delivered to Peers";
      description = "NFS read throughput against qBittorrent upload — the gap is read amplification.";
      type = "timeseries";
      id = 11;
      gridPos = {
        h = 8;
        w = 12;
        x = 12;
        y = 4;
      };
      targets = [
        {
          datasource = prom;
          expr = "sum(rate(node_mountstats_nfs_total_read_bytes_total${mount}[5m]))";
          legendFormat = "NFS read";
          refId = "A";
        }
        {
          datasource = prom;
          expr = "qbittorrent_global_upload_speed_bytes";
          legendFormat = "qbt upload";
          refId = "B";
        }
      ];
      fieldConfig.defaults.unit = "Bps";
    }

    # ── Operations ────────────────────────────────────────────────────────
    {
      title = "Operations by Type";
      type = "timeseries";
      id = 20;
      gridPos = {
        h = 8;
        w = 12;
        x = 0;
        y = 12;
      };
      targets = [
        {
          datasource = prom;
          expr = "sum by (operation) (rate(node_mountstats_nfs_operations_requests_total${mount}[5m]))";
          legendFormat = "{{operation}}";
          refId = "A";
        }
      ];
      fieldConfig.defaults.unit = "ops";
    }
    {
      title = "Average Bytes per READ";
      description = "Read bytes divided by READ ops — how much each round trip actually carries.";
      type = "timeseries";
      id = 21;
      gridPos = {
        h = 8;
        w = 12;
        x = 12;
        y = 12;
      };
      targets = [
        {
          datasource = prom;
          expr = ''sum(rate(node_mountstats_nfs_total_read_bytes_total${mount}[5m])) / sum(rate(node_mountstats_nfs_operations_requests_total{host="$host",export=~"$export",operation="READ"}[5m]))'';
          legendFormat = "bytes/read";
          refId = "A";
        }
      ];
      fieldConfig.defaults.unit = "bytes";
    }

    # ── Latency ───────────────────────────────────────────────────────────
    {
      title = "Response Time by Operation";
      type = "timeseries";
      id = 30;
      gridPos = {
        h = 8;
        w = 12;
        x = 0;
        y = 20;
      };
      targets = [
        {
          datasource = prom;
          expr = "sum by (operation) (rate(node_mountstats_nfs_operations_response_time_seconds_total${mount}[5m])) / sum by (operation) (rate(node_mountstats_nfs_operations_requests_total${mount}[5m]))";
          legendFormat = "{{operation}}";
          refId = "A";
        }
      ];
      fieldConfig.defaults.unit = "s";
    }
    {
      title = "Queue Time by Operation";
      description = "Time spent waiting for an RPC slot before transmission — rises when the client saturates the mount.";
      type = "timeseries";
      id = 31;
      gridPos = {
        h = 8;
        w = 12;
        x = 12;
        y = 20;
      };
      targets = [
        {
          datasource = prom;
          expr = "sum by (operation) (rate(node_mountstats_nfs_operations_queue_time_seconds_total${mount}[5m])) / sum by (operation) (rate(node_mountstats_nfs_operations_requests_total${mount}[5m]))";
          legendFormat = "{{operation}}";
          refId = "A";
        }
      ];
      fieldConfig.defaults.unit = "s";
    }

    # ── Transport ─────────────────────────────────────────────────────────
    {
      title = "RPC Backlog & Pending Queue";
      type = "timeseries";
      id = 40;
      gridPos = {
        h = 8;
        w = 12;
        x = 0;
        y = 28;
      };
      targets = [
        {
          datasource = prom;
          expr = "rate(node_mountstats_nfs_transport_backlog_queue_total${mount}[5m])";
          legendFormat = "backlog {{export}}";
          refId = "A";
        }
        {
          datasource = prom;
          expr = "rate(node_mountstats_nfs_transport_pending_queue_total${mount}[5m])";
          legendFormat = "pending {{export}}";
          refId = "B";
        }
      ];
      fieldConfig.defaults.unit = "short";
    }
    {
      title = "Max RPC Slots Used";
      type = "timeseries";
      id = 41;
      gridPos = {
        h = 8;
        w = 12;
        x = 12;
        y = 28;
      };
      targets = [
        {
          datasource = prom;
          expr = "node_mountstats_nfs_transport_maximum_rpc_slots${mount}";
          legendFormat = "{{export}}";
          refId = "A";
        }
      ];
      fieldConfig.defaults.unit = "short";
    }

    # ── VFS events ────────────────────────────────────────────────────────
    {
      title = "Page Reads (mmap vs readahead)";
      description = "vfs_read_page is a single-page fault; vfs_read_pages is a readahead batch.";
      type = "timeseries";
      id = 50;
      gridPos = {
        h = 8;
        w = 12;
        x = 0;
        y = 36;
      };
      targets = [
        {
          datasource = prom;
          expr = "rate(node_mountstats_nfs_event_vfs_read_page_total${mount}[5m])";
          legendFormat = "read_page {{export}}";
          refId = "A";
        }
        {
          datasource = prom;
          expr = "rate(node_mountstats_nfs_event_vfs_read_pages_total${mount}[5m])";
          legendFormat = "read_pages {{export}}";
          refId = "B";
        }
      ];
      fieldConfig.defaults.unit = "ops";
    }
    {
      title = "Errors, Timeouts & Short Reads";
      type = "timeseries";
      id = 51;
      gridPos = {
        h = 8;
        w = 12;
        x = 12;
        y = 36;
      };
      targets = [
        {
          datasource = prom;
          expr = "sum(rate(node_mountstats_nfs_operations_major_timeouts_total${mount}[5m]))";
          legendFormat = "major timeouts";
          refId = "A";
        }
        {
          datasource = prom;
          expr = "sum(rate(node_mountstats_nfs_event_short_read_total${mount}[5m]))";
          legendFormat = "short reads";
          refId = "B";
        }
        {
          datasource = prom;
          expr = "sum(rate(node_mountstats_nfs_event_data_invalidate_total${mount}[5m]))";
          legendFormat = "data invalidate";
          refId = "C";
        }
        {
          datasource = prom;
          expr = "sum(rate(node_mountstats_nfs_event_attribute_invalidate_total${mount}[5m]))";
          legendFormat = "attribute invalidate";
          refId = "D";
        }
      ];
      fieldConfig.defaults.unit = "ops";
    }

    # ── FS-Cache (client read cache on nvme) ──────────────────────────────
    # fscache is global (one cache, no per-mount export label) → keyed on host only.
    # Metrics come from the fscache-textfile exporter (server disks.nix) via node-exporter.
    {
      title = "Cache Hit Ratio";
      description = "fscache reads served from local nvme vs fetched from the NAS. Climbs as the seed set warms; ~0 right after a cache flush.";
      type = "stat";
      id = 60;
      gridPos = {
        h = 4;
        w = 6;
        x = 0;
        y = 44;
      };
      targets = [
        {
          datasource = prom;
          expr = ''sum(rate(fscache_read_hits_total{host="$host"}[5m])) / clamp_min(sum(rate(fscache_read_hits_total{host="$host"}[5m])) + sum(rate(fscache_downloads_total{host="$host"}[5m])), 1)'';
          refId = "A";
        }
      ];
      options = {
        reduceOptions.calcs = [ "lastNotNull" ];
        colorMode = "background";
      };
      fieldConfig.defaults = {
        unit = "percentunit";
        min = 0;
        max = 1;
        thresholds = {
          mode = "absolute";
          steps = [
            {
              color = "red";
              value = null;
            }
            {
              color = "yellow";
              value = 0.25;
            }
            {
              color = "green";
              value = 0.6;
            }
          ];
        };
      };
    }
    {
      title = "Cached Objects";
      type = "stat";
      id = 61;
      gridPos = {
        h = 4;
        w = 6;
        x = 6;
        y = 44;
      };
      targets = [
        {
          datasource = prom;
          expr = ''sum(fscache_cookies{host="$host"})'';
          refId = "A";
        }
      ];
      options = {
        reduceOptions.calcs = [ "lastNotNull" ];
        colorMode = "background";
      };
      fieldConfig.defaults.unit = "short";
    }
    {
      title = "Cache Hits/s";
      type = "stat";
      id = 62;
      gridPos = {
        h = 4;
        w = 6;
        x = 12;
        y = 44;
      };
      targets = [
        {
          datasource = prom;
          expr = ''sum(rate(fscache_read_hits_total{host="$host"}[5m]))'';
          refId = "A";
        }
      ];
      options = {
        reduceOptions.calcs = [ "lastNotNull" ];
        colorMode = "background";
      };
      fieldConfig.defaults.unit = "ops";
    }
    {
      title = "Cache Stores/s";
      type = "stat";
      id = 63;
      gridPos = {
        h = 4;
        w = 6;
        x = 18;
        y = 44;
      };
      targets = [
        {
          datasource = prom;
          expr = ''sum(rate(fscache_writes_total{host="$host"}[5m]))'';
          refId = "A";
        }
      ];
      options = {
        reduceOptions.calcs = [ "lastNotNull" ];
        colorMode = "background";
      };
      fieldConfig.defaults.unit = "ops";
    }
    {
      title = "Cache Hits vs NAS Fetches";
      description = "Reads served from the nvme cache vs reads that fell through to the UNAS. As the cache warms, hits should rise and NAS fetches fall for repeat content.";
      type = "timeseries";
      id = 64;
      gridPos = {
        h = 8;
        w = 12;
        x = 0;
        y = 48;
      };
      targets = [
        {
          datasource = prom;
          expr = ''sum(rate(fscache_read_hits_total{host="$host"}[5m]))'';
          legendFormat = "cache hits";
          refId = "A";
        }
        {
          datasource = prom;
          expr = ''sum(rate(fscache_downloads_total{host="$host"}[5m]))'';
          legendFormat = "NAS fetches";
          refId = "B";
        }
      ];
      fieldConfig.defaults.unit = "ops";
    }
    {
      title = "Hit Ratio Over Time";
      type = "timeseries";
      id = 65;
      gridPos = {
        h = 8;
        w = 12;
        x = 12;
        y = 48;
      };
      targets = [
        {
          datasource = prom;
          expr = ''sum(rate(fscache_read_hits_total{host="$host"}[5m])) / clamp_min(sum(rate(fscache_read_hits_total{host="$host"}[5m])) + sum(rate(fscache_downloads_total{host="$host"}[5m])), 1)'';
          legendFormat = "hit ratio";
          refId = "A";
        }
      ];
      fieldConfig.defaults = {
        unit = "percentunit";
        min = 0;
        max = 1;
      };
    }

    # ── Cache disk capacity (dedicated WD SN580, xfs) ─────────────────────
    # host_type="host" so container sidecars reporting the same bind-mounted path don't
    # double the series. cachefilesd culls at ~93% used and hard-stops ~97%, so 90%+ is
    # normal steady state, not a problem — thresholds coloured accordingly.
    {
      title = "Cache Disk Used %";
      type = "stat";
      id = 66;
      gridPos = {
        h = 4;
        w = 8;
        x = 0;
        y = 56;
      };
      targets = [
        {
          datasource = prom;
          expr = ''1 - (node_filesystem_avail_bytes{mountpoint="/var/cache/fscache",host="$host",host_type="host"} / node_filesystem_size_bytes{mountpoint="/var/cache/fscache",host="$host",host_type="host"})'';
          refId = "A";
        }
      ];
      options = {
        reduceOptions.calcs = [ "lastNotNull" ];
        colorMode = "background";
      };
      fieldConfig.defaults = {
        unit = "percentunit";
        min = 0;
        max = 1;
        thresholds = {
          mode = "absolute";
          steps = [
            {
              color = "green";
              value = null;
            }
            {
              color = "yellow";
              value = 0.9;
            }
            {
              color = "red";
              value = 0.96;
            }
          ];
        };
      };
    }
    {
      title = "Cache Disk Used";
      type = "stat";
      id = 67;
      gridPos = {
        h = 4;
        w = 8;
        x = 8;
        y = 56;
      };
      targets = [
        {
          datasource = prom;
          expr = ''node_filesystem_size_bytes{mountpoint="/var/cache/fscache",host="$host",host_type="host"} - node_filesystem_avail_bytes{mountpoint="/var/cache/fscache",host="$host",host_type="host"}'';
          refId = "A";
        }
      ];
      options = {
        reduceOptions.calcs = [ "lastNotNull" ];
        colorMode = "background";
      };
      fieldConfig.defaults.unit = "bytes";
    }
    {
      title = "Cache Disk Free";
      type = "stat";
      id = 68;
      gridPos = {
        h = 4;
        w = 8;
        x = 16;
        y = 56;
      };
      targets = [
        {
          datasource = prom;
          expr = ''node_filesystem_avail_bytes{mountpoint="/var/cache/fscache",host="$host",host_type="host"}'';
          refId = "A";
        }
      ];
      options = {
        reduceOptions.calcs = [ "lastNotNull" ];
        colorMode = "background";
      };
      fieldConfig.defaults.unit = "bytes";
    }
    {
      title = "Cache Disk Usage";
      description = "Bytes used on the dedicated fscache drive. Ramps as the cache warms, then plateaus/sawtooths as cachefilesd culls (LRU) at the high-water mark.";
      type = "timeseries";
      id = 69;
      gridPos = {
        h = 8;
        w = 24;
        x = 0;
        y = 60;
      };
      targets = [
        {
          datasource = prom;
          expr = ''node_filesystem_size_bytes{mountpoint="/var/cache/fscache",host="$host",host_type="host"} - node_filesystem_avail_bytes{mountpoint="/var/cache/fscache",host="$host",host_type="host"}'';
          legendFormat = "used";
          refId = "A";
        }
      ];
      fieldConfig.defaults.unit = "bytes";
    }
  ];
}
