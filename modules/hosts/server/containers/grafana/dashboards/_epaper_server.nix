# Server overview for the reTerminal E1003 e-paper panel. Rendered by
# grafana-image-renderer at 936x702 @2x (1872x1404 output) and shown in
# 16-level grayscale, so nothing here relies on color: large values, thick
# lines, no legends. The datasource uid is the real one rather than a name
# ref, since the renderer is a strict context where name fallback breaks.
let
  prom = {
    type = "prometheus";
    uid = "PBFA97CFB590B2093";
  };

  target = expr: legend: {
    datasource = prom;
    inherit expr;
    legendFormat = legend;
    refId = legend;
  };

  stat =
    {
      id,
      title,
      x,
      expr,
      unit,
      decimals ? 0,
    }:
    {
      inherit id title;
      type = "stat";
      datasource = prom;
      gridPos = {
        h = 5;
        w = 4;
        inherit x;
        y = 0;
      };
      targets = [ (target expr "A") ];
      options = {
        colorMode = "none";
        graphMode = "none";
        justifyMode = "center";
        textMode = "value";
        text.valueSize = 56;
        reduceOptions = {
          calcs = [ "lastNotNull" ];
          fields = "";
          values = false;
        };
      };
      fieldConfig.defaults = {
        inherit unit decimals;
        color.mode = "fixed";
        color.fixedColor = "black";
      };
    };

  timeseries =
    {
      id,
      title,
      gridPos,
      targets,
      unit,
      min ? null,
      max ? null,
    }:
    {
      inherit
        id
        title
        gridPos
        targets
        ;
      type = "timeseries";
      datasource = prom;
      options = {
        legend.showLegend = false;
        tooltip.mode = "none";
      };
      fieldConfig.defaults = {
        inherit unit;
        color.mode = "fixed";
        color.fixedColor = "black";
        custom = {
          lineWidth = 3;
          fillOpacity = 0;
          showPoints = "never";
          axisBorderShow = true;
        };
      }
      // (if min == null then { } else { inherit min; })
      // (if max == null then { } else { inherit max; });
    };

  # rx above the axis, tx mirrored below, so both fit one black series style.
  rxtx = rx: tx: [
    (target rx "rx")
    (target "-(${tx})" "tx")
  ];
in
{
  folder = "Infrastructure";
  title = "E-Paper Server";
  uid = "epaper-server";
  tags = [
    "epaper"
    "infra"
  ];
  editable = true;
  schemaVersion = 38;
  time = {
    from = "now-6h";
    to = "now";
  };
  panels = [
    (stat {
      id = 1;
      title = "CPU %";
      x = 0;
      expr = ''100 - avg(rate(node_cpu_seconds_total{host="server",mode="idle"}[5m])) * 100'';
      unit = "none";
    })
    (stat {
      id = 2;
      title = "Memory %";
      x = 4;
      expr = ''100 * (1 - node_memory_MemAvailable_bytes{host="server"} / node_memory_MemTotal_bytes{host="server"})'';
      unit = "none";
    })
    (stat {
      id = 3;
      title = "CPU Temp °C";
      x = 8;
      expr = ''node_hwmon_temp_celsius{host="server",chip="pci0000:00_0000:00:18_3",sensor="temp1"}'';
      unit = "none";
    })
    (stat {
      id = 4;
      title = "Root %";
      x = 12;
      expr = ''100 * (1 - node_filesystem_avail_bytes{host="server",mountpoint="/"} / node_filesystem_size_bytes{host="server",mountpoint="/"})'';
      unit = "none";
    })
    (stat {
      id = 5;
      title = "NAS %";
      x = 16;
      expr = ''100 * (1 - node_filesystem_avail_bytes{host="server",mountpoint="/mnt/data"} / node_filesystem_size_bytes{host="server",mountpoint="/mnt/data"})'';
      unit = "none";
    })
    (stat {
      id = 6;
      title = "Containers";
      x = 20;
      expr = ''count(node_systemd_unit_state{host="server",state="active",name=~"container@.*"} == 1)'';
      unit = "none";
    })
    (timeseries {
      id = 7;
      title = "CPU %";
      gridPos = {
        h = 7;
        w = 12;
        x = 0;
        y = 5;
      };
      targets = [
        (target ''100 - avg(rate(node_cpu_seconds_total{host="server",mode="idle"}[5m])) * 100'' "cpu")
      ];
      unit = "percent";
      min = 0;
      max = 100;
    })
    (timeseries {
      id = 8;
      title = "Memory %";
      gridPos = {
        h = 7;
        w = 12;
        x = 12;
        y = 5;
      };
      targets = [
        (target ''100 * (1 - node_memory_MemAvailable_bytes{host="server"} / node_memory_MemTotal_bytes{host="server"})'' "mem")
      ];
      unit = "percent";
      min = 0;
      max = 100;
    })
    {
      id = 9;
      title = "Failed Units";
      type = "table";
      datasource = prom;
      gridPos = {
        h = 6;
        w = 8;
        x = 0;
        y = 12;
      };
      targets = [
        (
          target ''node_systemd_unit_state{host="server",state="failed"} == 1'' "failed"
          // {
            format = "table";
            instant = true;
          }
        )
      ];
      options = {
        showHeader = false;
        cellHeight = "sm";
      };
      fieldConfig.defaults = {
        custom.align = "left";
        noValue = "All units OK";
      };
      # Only the unit name matters; everything else is label noise.
      transformations = [
        {
          id = "organize";
          options.excludeByName = {
            Time = true;
            Value = true;
            __name__ = true;
            host = true;
            host_type = true;
            instance = true;
            job = true;
            state = true;
            type = true;
          };
        }
      ];
    }
    (timeseries {
      id = 10;
      title = "WAN";
      gridPos = {
        h = 6;
        w = 8;
        x = 8;
        y = 12;
      };
      targets = rxtx ''sum(unpoller_device_wan_receive_rate_bytes{host_type="network"})'' ''sum(unpoller_device_wan_transmit_rate_bytes{host_type="network"})'';
      unit = "Bps";
    })
    (timeseries {
      id = 11;
      title = "bond0";
      gridPos = {
        h = 6;
        w = 8;
        x = 16;
        y = 12;
      };
      targets = rxtx ''rate(node_network_receive_bytes_total{host="server",device="bond0"}[5m])'' ''rate(node_network_transmit_bytes_total{host="server",device="bond0"}[5m])'';
      unit = "Bps";
    })
  ];
}
