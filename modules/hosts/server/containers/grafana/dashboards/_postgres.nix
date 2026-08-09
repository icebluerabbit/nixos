{
  folder = "Infrastructure";
  title = "PostgreSQL";
  tags = [
    "infra"
    "database"
    "postgres"
  ];
  uid = "postgresql";
  editable = true;
  schemaVersion = 38;
  refresh = "30s";
  time = {
    from = "now-1h";
    to = "now";
  };
  panels = [

    # ── Overview stats ────────────────────────────────────────────────────
    {
      title = "Total Connections";
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
          datasource = {
            type = "prometheus";
            uid = "Prometheus";
          };
          expr = ''sum(pg_stat_database_numbackends{host="postgresql",datname!~"template.*|postgres"})'';
          refId = "A";
        }
      ];
      options = {
        reduceOptions.calcs = [ "lastNotNull" ];
        colorMode = "value";
        graphMode = "area";
      };
      fieldConfig.defaults = {
        unit = "short";
        decimals = 0;
      };
    }

    {
      title = "Active Queries";
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
          datasource = {
            type = "prometheus";
            uid = "Prometheus";
          };
          expr = ''sum(pg_stat_activity_count{host="postgresql",datname!~"template.*|postgres",state="active"})'';
          refId = "A";
        }
      ];
      options = {
        reduceOptions.calcs = [ "lastNotNull" ];
        colorMode = "value";
        graphMode = "area";
      };
      fieldConfig.defaults = {
        unit = "short";
        decimals = 0;
      };
    }

    {
      title = "Idle in Transaction";
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
          datasource = {
            type = "prometheus";
            uid = "Prometheus";
          };
          expr = ''sum(pg_stat_activity_count{host="postgresql",datname!~"template.*|postgres",state="idle in transaction"})'';
          refId = "A";
        }
      ];
      options = {
        reduceOptions.calcs = [ "lastNotNull" ];
        colorMode = "background";
      };
      fieldConfig = {
        defaults = {
          unit = "short";
          decimals = 0;
          thresholds = {
            mode = "absolute";
            steps = [
              {
                color = "green";
                value = null;
              }
              {
                color = "yellow";
                value = 1;
              }
              {
                color = "red";
                value = 5;
              }
            ];
          };
        };
      };
    }

    {
      title = "WAL Size";
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
          datasource = {
            type = "prometheus";
            uid = "Prometheus";
          };
          expr = ''pg_wal_size_bytes{host="postgresql"}'';
          refId = "A";
        }
      ];
      options = {
        reduceOptions.calcs = [ "lastNotNull" ];
        colorMode = "value";
      };
      fieldConfig.defaults.unit = "bytes";
    }

    # ── Connections ───────────────────────────────────────────────────────
    {
      title = "Connections per Database";
      type = "timeseries";
      id = 5;
      gridPos = {
        h = 8;
        w = 12;
        x = 0;
        y = 4;
      };
      targets = [
        {
          datasource = {
            type = "prometheus";
            uid = "Prometheus";
          };
          expr = ''pg_stat_database_numbackends{host="postgresql",datname!~"template.*|postgres"}'';
          legendFormat = "{{datname}}";
          refId = "A";
        }
      ];
      fieldConfig.defaults = {
        unit = "short";
        decimals = 0;
        min = 0;
      };
    }

    {
      title = "Connection States";
      type = "timeseries";
      id = 6;
      gridPos = {
        h = 8;
        w = 12;
        x = 12;
        y = 4;
      };
      targets = [
        {
          datasource = {
            type = "prometheus";
            uid = "Prometheus";
          };
          expr = ''sum by (state) (pg_stat_activity_count{host="postgresql",datname!~"template.*|postgres",state!="disabled"})'';
          legendFormat = "{{state}}";
          refId = "A";
        }
      ];
      fieldConfig.defaults = {
        unit = "short";
        decimals = 0;
        min = 0;
      };
    }

    # ── Database Sizes ────────────────────────────────────────────────────
    {
      title = "Database Sizes";
      type = "bargauge";
      id = 7;
      gridPos = {
        h = 8;
        w = 24;
        x = 0;
        y = 12;
      };
      targets = [
        {
          datasource = {
            type = "prometheus";
            uid = "Prometheus";
          };
          expr = ''sort_desc(pg_database_size_bytes{host="postgresql",datname!~"template.*|postgres"})'';
          legendFormat = "{{datname}}";
          refId = "A";
          instant = true;
        }
      ];
      options = {
        orientation = "horizontal";
        reduceOptions.calcs = [ "lastNotNull" ];
        displayMode = "gradient";
      };
      fieldConfig.defaults = {
        unit = "bytes";
        min = 0;
      };
    }

    # ── Performance ───────────────────────────────────────────────────────
    {
      title = "Cache Hit Ratio";
      type = "timeseries";
      id = 8;
      gridPos = {
        h = 8;
        w = 12;
        x = 0;
        y = 20;
      };
      targets = [
        {
          datasource = {
            type = "prometheus";
            uid = "Prometheus";
          };
          expr = ''rate(pg_stat_database_blks_hit{host="postgresql",datname!~"template.*|postgres"}[5m]) / (rate(pg_stat_database_blks_hit{host="postgresql",datname!~"template.*|postgres"}[5m]) + rate(pg_stat_database_blks_read{host="postgresql",datname!~"template.*|postgres"}[5m]))'';
          legendFormat = "{{datname}}";
          refId = "A";
        }
      ];
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
              value = 0.9;
            }
            {
              color = "green";
              value = 0.99;
            }
          ];
        };
      };
    }

    {
      title = "Transactions per Second";
      type = "timeseries";
      id = 9;
      gridPos = {
        h = 8;
        w = 12;
        x = 12;
        y = 20;
      };
      targets = [
        {
          datasource = {
            type = "prometheus";
            uid = "Prometheus";
          };
          expr = ''sum by (datname) (rate(pg_stat_database_xact_commit{host="postgresql",datname!~"template.*|postgres"}[5m]) + rate(pg_stat_database_xact_rollback{host="postgresql",datname!~"template.*|postgres"}[5m]))'';
          legendFormat = "{{datname}}";
          refId = "A";
        }
      ];
      fieldConfig.defaults = {
        unit = "ops";
        min = 0;
      };
    }

    # ── Tuple Operations ──────────────────────────────────────────────────
    {
      title = "Tuple Inserts/sec";
      type = "timeseries";
      id = 10;
      gridPos = {
        h = 8;
        w = 8;
        x = 0;
        y = 28;
      };
      targets = [
        {
          datasource = {
            type = "prometheus";
            uid = "Prometheus";
          };
          expr = ''sum by (datname) (rate(pg_stat_database_tup_inserted{host="postgresql",datname!~"template.*|postgres"}[5m]))'';
          legendFormat = "{{datname}}";
          refId = "A";
        }
      ];
      fieldConfig.defaults = {
        unit = "ops";
        min = 0;
      };
    }

    {
      title = "Tuple Updates/sec";
      type = "timeseries";
      id = 11;
      gridPos = {
        h = 8;
        w = 8;
        x = 8;
        y = 28;
      };
      targets = [
        {
          datasource = {
            type = "prometheus";
            uid = "Prometheus";
          };
          expr = ''sum by (datname) (rate(pg_stat_database_tup_updated{host="postgresql",datname!~"template.*|postgres"}[5m]))'';
          legendFormat = "{{datname}}";
          refId = "A";
        }
      ];
      fieldConfig.defaults = {
        unit = "ops";
        min = 0;
      };
    }

    {
      title = "Tuple Deletes/sec";
      type = "timeseries";
      id = 12;
      gridPos = {
        h = 8;
        w = 8;
        x = 16;
        y = 28;
      };
      targets = [
        {
          datasource = {
            type = "prometheus";
            uid = "Prometheus";
          };
          expr = ''sum by (datname) (rate(pg_stat_database_tup_deleted{host="postgresql",datname!~"template.*|postgres"}[5m]))'';
          legendFormat = "{{datname}}";
          refId = "A";
        }
      ];
      fieldConfig.defaults = {
        unit = "ops";
        min = 0;
      };
    }

    # ── Health ────────────────────────────────────────────────────────────
    {
      title = "Locks by Mode";
      type = "bargauge";
      id = 13;
      gridPos = {
        h = 8;
        w = 12;
        x = 0;
        y = 36;
      };
      targets = [
        {
          datasource = {
            type = "prometheus";
            uid = "Prometheus";
          };
          expr = ''sum by (mode) (pg_locks_count{host="postgresql"})'';
          legendFormat = "{{mode}}";
          refId = "A";
          instant = true;
        }
      ];
      options = {
        orientation = "horizontal";
        reduceOptions.calcs = [ "lastNotNull" ];
        displayMode = "basic";
      };
      fieldConfig.defaults = {
        unit = "short";
        decimals = 0;
        min = 0;
      };
    }

    {
      title = "Deadlock Rate";
      type = "timeseries";
      id = 14;
      gridPos = {
        h = 8;
        w = 12;
        x = 12;
        y = 36;
      };
      targets = [
        {
          datasource = {
            type = "prometheus";
            uid = "Prometheus";
          };
          expr = ''rate(pg_stat_database_deadlocks{host="postgresql",datname!~"template.*|postgres"}[5m])'';
          legendFormat = "{{datname}}";
          refId = "A";
        }
      ];
      fieldConfig = {
        defaults = {
          unit = "ops";
          min = 0;
          thresholds = {
            mode = "absolute";
            steps = [
              {
                color = "green";
                value = null;
              }
              {
                color = "red";
                value = 0.001;
              }
            ];
          };
        };
      };
    }

  ];
}
