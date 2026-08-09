let
  prom = {
    type = "prometheus";
    uid = "Prometheus";
  };
  loki = {
    type = "loki";
    uid = "Loki";
  };
in
{
  folder = "IoT";
  title = "Mosquitto";
  uid = "mosquitto";
  tags = [
    "iot"
    "mqtt"
    "mosquitto"
    "broker"
  ];
  editable = true;
  schemaVersion = 38;
  refresh = "30s";
  time = {
    from = "now-6h";
    to = "now";
  };
  panels = [

    # ── KPIs ──────────────────────────────────────────────────────────────
    {
      title = "Connected Clients";
      type = "stat";
      id = 1;
      gridPos = {
        h = 4;
        w = 5;
        x = 0;
        y = 0;
      };
      targets = [
        {
          datasource = prom;
          expr = "mosquitto_clients_connected";
          refId = "A";
        }
      ];
      options = {
        reduceOptions.calcs = [ "lastNotNull" ];
        colorMode = "background";
      };
      fieldConfig.defaults = {
        unit = "short";
        decimals = 0;
      };
    }
    {
      title = "Total Clients";
      type = "stat";
      id = 2;
      gridPos = {
        h = 4;
        w = 5;
        x = 5;
        y = 0;
      };
      targets = [
        {
          datasource = prom;
          expr = "mosquitto_clients_total";
          refId = "A";
        }
      ];
      options.reduceOptions.calcs = [ "lastNotNull" ];
      fieldConfig.defaults = {
        unit = "short";
        decimals = 0;
      };
    }
    {
      title = "Peak Clients";
      type = "stat";
      id = 3;
      gridPos = {
        h = 4;
        w = 5;
        x = 10;
        y = 0;
      };
      targets = [
        {
          datasource = prom;
          expr = "mosquitto_clients_maximum";
          refId = "A";
        }
      ];
      options.reduceOptions.calcs = [ "lastNotNull" ];
      fieldConfig.defaults = {
        unit = "short";
        decimals = 0;
      };
    }
    {
      title = "Retained Messages";
      type = "stat";
      id = 4;
      gridPos = {
        h = 4;
        w = 4;
        x = 15;
        y = 0;
      };
      targets = [
        {
          datasource = prom;
          expr = "mosquitto_retained_messages_count";
          refId = "A";
        }
      ];
      options.reduceOptions.calcs = [ "lastNotNull" ];
      fieldConfig.defaults = {
        unit = "short";
        decimals = 0;
      };
    }
    {
      title = "Broker Heap";
      type = "stat";
      id = 5;
      gridPos = {
        h = 4;
        w = 5;
        x = 19;
        y = 0;
      };
      targets = [
        {
          datasource = prom;
          expr = "mosquitto_heap_current";
          refId = "A";
        }
      ];
      options = {
        reduceOptions.calcs = [ "lastNotNull" ];
        colorMode = "value";
      };
      fieldConfig.defaults.unit = "bytes";
    }

    # ── Throughput ────────────────────────────────────────────────────────
    {
      title = "Message Load (per min)";
      type = "timeseries";
      id = 6;
      gridPos = {
        h = 8;
        w = 12;
        x = 0;
        y = 4;
      };
      targets = [
        {
          datasource = prom;
          expr = "mosquitto_load_messages_received_1min";
          legendFormat = "received";
          refId = "A";
        }
        {
          datasource = prom;
          expr = "mosquitto_load_messages_sent_1min";
          legendFormat = "sent";
          refId = "B";
        }
      ];
      fieldConfig.defaults = {
        unit = "short";
        custom = {
          drawStyle = "line";
          fillOpacity = 10;
          lineWidth = 1;
        };
      };
    }
    {
      title = "Byte Load (per min)";
      type = "timeseries";
      id = 7;
      gridPos = {
        h = 8;
        w = 12;
        x = 12;
        y = 4;
      };
      targets = [
        {
          datasource = prom;
          expr = "mosquitto_load_bytes_received_1min";
          legendFormat = "received";
          refId = "A";
        }
        {
          datasource = prom;
          expr = "mosquitto_load_bytes_sent_1min";
          legendFormat = "sent";
          refId = "B";
        }
      ];
      fieldConfig.defaults = {
        unit = "bytes";
        custom = {
          drawStyle = "line";
          fillOpacity = 10;
          lineWidth = 1;
        };
      };
    }

    # ── Clients + messages over time ──────────────────────────────────────
    {
      title = "Clients Connected";
      type = "timeseries";
      id = 8;
      gridPos = {
        h = 8;
        w = 12;
        x = 0;
        y = 12;
      };
      targets = [
        {
          datasource = prom;
          expr = "mosquitto_clients_connected";
          legendFormat = "connected";
          refId = "A";
        }
      ];
      fieldConfig.defaults = {
        unit = "short";
        custom.fillOpacity = 20;
      };
    }
    {
      title = "Messages Stored";
      type = "timeseries";
      id = 9;
      gridPos = {
        h = 8;
        w = 12;
        x = 12;
        y = 12;
      };
      targets = [
        {
          datasource = prom;
          expr = "mosquitto_messages_stored";
          legendFormat = "stored";
          refId = "A";
        }
      ];
      fieldConfig.defaults.unit = "short";
    }

    # ── Logs ──────────────────────────────────────────────────────────────
    {
      title = "Broker Logs";
      type = "logs";
      id = 10;
      gridPos = {
        h = 8;
        w = 24;
        x = 0;
        y = 20;
      };
      targets = [
        {
          datasource = loki;
          expr = ''{host="mosquitto", host_type="container"}'';
          refId = "A";
        }
      ];
      options = {
        showTime = true;
        sortOrder = "Descending";
        wrapLogMessage = true;
      };
    }

  ];
}
