let
  prom = {
    type = "prometheus";
    uid = "Prometheus";
  };
  loki = {
    type = "loki";
    uid = "Loki";
  };
  h = ''host="jellyfin"'';
in
{
  folder = "Media";
  title = "Jellyfin";
  tags = [
    "media"
    "jellyfin"
  ];
  uid = "jellyfin";
  editable = true;
  schemaVersion = 38;
  refresh = "30s";
  time = {
    from = "now-24h";
    to = "now";
  };
  panels = [

    # ── Library KPIs ──────────────────────────────────────────────────────
    {
      title = "Movies";
      type = "stat";
      id = 1;
      gridPos = {
        h = 4;
        w = 4;
        x = 0;
        y = 0;
      };
      targets = [
        {
          datasource = prom;
          expr = "jellyfin_library_movies{${h}}";
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
      title = "Series";
      type = "stat";
      id = 2;
      gridPos = {
        h = 4;
        w = 4;
        x = 4;
        y = 0;
      };
      targets = [
        {
          datasource = prom;
          expr = "jellyfin_library_series{${h}}";
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
      title = "Episodes";
      type = "stat";
      id = 3;
      gridPos = {
        h = 4;
        w = 4;
        x = 8;
        y = 0;
      };
      targets = [
        {
          datasource = prom;
          expr = "jellyfin_library_episodes{${h}}";
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
      title = "Albums";
      type = "stat";
      id = 4;
      gridPos = {
        h = 4;
        w = 4;
        x = 12;
        y = 0;
      };
      targets = [
        {
          datasource = prom;
          expr = "jellyfin_library_albums{${h}}";
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
      title = "Songs";
      type = "stat";
      id = 5;
      gridPos = {
        h = 4;
        w = 4;
        x = 16;
        y = 0;
      };
      targets = [
        {
          datasource = prom;
          expr = "jellyfin_library_songs{${h}}";
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
      title = "Up";
      type = "stat";
      id = 6;
      gridPos = {
        h = 4;
        w = 4;
        x = 20;
        y = 0;
      };
      targets = [
        {
          datasource = prom;
          expr = "jellyfin_up{${h}}";
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
        mappings = [
          {
            type = "value";
            options = {
              "0".text = "DOWN";
              "1".text = "UP";
            };
          }
        ];
        thresholds = {
          mode = "absolute";
          steps = [
            {
              color = "red";
              value = null;
            }
            {
              color = "green";
              value = 1;
            }
          ];
        };
      };
    }

    # ── Library over time ─────────────────────────────────────────────────
    {
      title = "Library Counts";
      type = "timeseries";
      id = 7;
      gridPos = {
        h = 8;
        w = 12;
        x = 0;
        y = 4;
      };
      targets = [
        {
          datasource = prom;
          expr = "jellyfin_library_movies{${h}}";
          legendFormat = "movies";
          refId = "A";
        }
        {
          datasource = prom;
          expr = "jellyfin_library_series{${h}}";
          legendFormat = "series";
          refId = "B";
        }
        {
          datasource = prom;
          expr = "jellyfin_library_episodes{${h}}";
          legendFormat = "episodes";
          refId = "C";
        }
        {
          datasource = prom;
          expr = "jellyfin_library_albums{${h}}";
          legendFormat = "albums";
          refId = "D";
        }
        {
          datasource = prom;
          expr = "jellyfin_library_songs{${h}}";
          legendFormat = "songs";
          refId = "E";
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

    # ── Playback activity ─────────────────────────────────────────────────
    {
      title = "Active Sessions / Streams / Transcodes";
      type = "timeseries";
      id = 8;
      gridPos = {
        h = 8;
        w = 12;
        x = 12;
        y = 4;
      };
      targets = [
        {
          datasource = prom;
          expr = "jellyfin_active_sessions{${h}}";
          legendFormat = "sessions";
          refId = "A";
        }
        {
          datasource = prom;
          expr = "jellyfin_active_streams{${h}}";
          legendFormat = "streams";
          refId = "B";
        }
        {
          datasource = prom;
          expr = "jellyfin_transcodes{${h}}";
          legendFormat = "transcodes";
          refId = "C";
        }
      ];
      fieldConfig.defaults = {
        unit = "short";
        decimals = 0;
        custom = {
          drawStyle = "line";
          fillOpacity = 10;
          lineWidth = 1;
        };
      };
    }

    # ── Logs ──────────────────────────────────────────────────────────────
    {
      title = "Warnings / Errors / Playback";
      type = "logs";
      id = 9;
      gridPos = {
        h = 9;
        w = 24;
        x = 0;
        y = 12;
      };
      targets = [
        {
          datasource = loki;
          expr = ''{host="jellyfin", host_type="container"} |~ "(?i)\\[ERR\\]|\\[WRN\\]|Playback|session"'';
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
