{
  folder = "IoT";
  title = "Home Sensors";
  uid = "home-sensors";
  tags = [
    "iot"
    "hass"
    "sensors"
    "zigbee"
  ];
  editable = true;
  schemaVersion = 38;
  refresh = "1m";
  time = {
    from = "now-24h";
    to = "now";
  };
  panels = [
    {
      id = 1;
      type = "stat";
      title = "Temperature";
      gridPos = {
        x = 0;
        y = 0;
        w = 12;
        h = 4;
      };
      options = {
        colorMode = "background";
        graphMode = "none";
        reduceOptions = {
          calcs = [ "lastNotNull" ];
          fields = "";
          values = false;
        };
      };
      fieldConfig = {
        defaults = {
          unit = "fahrenheit";
          thresholds = {
            mode = "absolute";
            steps = [
              {
                color = "blue";
                value = null;
              }
              {
                color = "green";
                value = 64;
              }
              {
                color = "yellow";
                value = 79;
              }
              {
                color = "red";
                value = 90;
              }
            ];
          };
        };
      };
      targets = [
        {
          datasource = {
            type = "prometheus";
            uid = "Prometheus";
          };
          expr = "(homeassistant_sensor_temperature_celsius{entity=\"sensor.bedroom_temp_sensor_temperature\"} * 9/5) + 32";
          legendFormat = "Bedroom";
          refId = "A";
        }
        {
          datasource = {
            type = "prometheus";
            uid = "Prometheus";
          };
          expr = "(homeassistant_sensor_temperature_celsius{entity=\"sensor.living_room_temp_sensor_temperature\"} * 9/5) + 32";
          legendFormat = "Living Room";
          refId = "B";
        }
      ];
    }
    {
      id = 2;
      type = "stat";
      title = "Humidity";
      gridPos = {
        x = 12;
        y = 0;
        w = 12;
        h = 4;
      };
      options = {
        colorMode = "background";
        graphMode = "none";
        reduceOptions = {
          calcs = [ "lastNotNull" ];
          fields = "";
          values = false;
        };
      };
      fieldConfig = {
        defaults = {
          unit = "humidity";
          thresholds = {
            mode = "absolute";
            steps = [
              {
                color = "red";
                value = null;
              }
              {
                color = "green";
                value = 30;
              }
              {
                color = "blue";
                value = 60;
              }
            ];
          };
        };
      };
      targets = [
        {
          datasource = {
            type = "prometheus";
            uid = "Prometheus";
          };
          expr = "homeassistant_sensor_humidity_percent{entity=\"sensor.bedroom_temp_sensor_humidity\"}";
          legendFormat = "Bedroom";
          refId = "A";
        }
        {
          datasource = {
            type = "prometheus";
            uid = "Prometheus";
          };
          expr = "homeassistant_sensor_humidity_percent{entity=\"sensor.living_room_temp_sensor_humidity\"}";
          legendFormat = "Living Room";
          refId = "B";
        }
      ];
    }
    {
      id = 3;
      type = "timeseries";
      title = "Temperature History";
      gridPos = {
        x = 0;
        y = 8;
        w = 24;
        h = 8;
      };
      options = {
        tooltip = {
          mode = "multi";
        };
        legend = {
          displayMode = "list";
          placement = "bottom";
        };
      };
      fieldConfig = {
        defaults = {
          unit = "fahrenheit";
          custom = {
            lineWidth = 2;
            fillOpacity = 10;
          };
        };
      };
      targets = [
        {
          datasource = {
            type = "prometheus";
            uid = "Prometheus";
          };
          expr = "(homeassistant_sensor_temperature_celsius{entity=\"sensor.bedroom_temp_sensor_temperature\"} * 9/5) + 32";
          legendFormat = "Bedroom";
          refId = "A";
        }
        {
          datasource = {
            type = "prometheus";
            uid = "Prometheus";
          };
          expr = "(homeassistant_sensor_temperature_celsius{entity=\"sensor.living_room_temp_sensor_temperature\"} * 9/5) + 32";
          legendFormat = "Living Room";
          refId = "B";
        }
      ];
    }
    {
      id = 4;
      type = "timeseries";
      title = "Humidity History";
      gridPos = {
        x = 0;
        y = 16;
        w = 24;
        h = 8;
      };
      options = {
        tooltip = {
          mode = "multi";
        };
        legend = {
          displayMode = "list";
          placement = "bottom";
        };
      };
      fieldConfig = {
        defaults = {
          unit = "humidity";
          min = 0;
          max = 100;
          custom = {
            lineWidth = 2;
            fillOpacity = 10;
          };
        };
      };
      targets = [
        {
          datasource = {
            type = "prometheus";
            uid = "Prometheus";
          };
          expr = "homeassistant_sensor_humidity_percent{entity=\"sensor.bedroom_temp_sensor_humidity\"}";
          legendFormat = "Bedroom";
          refId = "A";
        }
        {
          datasource = {
            type = "prometheus";
            uid = "Prometheus";
          };
          expr = "homeassistant_sensor_humidity_percent{entity=\"sensor.living_room_temp_sensor_humidity\"}";
          legendFormat = "Living Room";
          refId = "B";
        }
      ];
    }
    {
      id = 5;
      type = "stat";
      title = "Presence";
      gridPos = {
        x = 0;
        y = 4;
        w = 12;
        h = 4;
      };
      options = {
        colorMode = "background";
        graphMode = "none";
        reduceOptions = {
          calcs = [ "lastNotNull" ];
          fields = "";
          values = false;
        };
      };
      fieldConfig = {
        defaults = {
          mappings = [
            {
              type = "value";
              options = {
                "0" = {
                  text = "Away";
                  color = "blue";
                };
                "1" = {
                  text = "Present";
                  color = "green";
                };
              };
            }
          ];
        };
      };
      targets = [
        {
          datasource = {
            type = "prometheus";
            uid = "Prometheus";
          };
          expr = "max(last_over_time(homeassistant_binary_sensor_state{entity=~\"binary_sensor.*96e8.*presence.*\"}[1h]))";
          legendFormat = "Bedroom";
          refId = "A";
        }
        {
          datasource = {
            type = "prometheus";
            uid = "Prometheus";
          };
          expr = "max(last_over_time(homeassistant_binary_sensor_state{entity=~\"binary_sensor.*db2e.*presence.*\"}[1h]))";
          legendFormat = "Living Room";
          refId = "B";
        }
      ];
    }
    {
      id = 6;
      type = "stat";
      title = "Light Level";
      gridPos = {
        x = 12;
        y = 4;
        w = 12;
        h = 4;
      };
      options = {
        colorMode = "background";
        graphMode = "none";
        reduceOptions = {
          calcs = [ "lastNotNull" ];
          fields = "";
          values = false;
        };
      };
      fieldConfig = {
        defaults = {
          unit = "lux";
          thresholds = {
            mode = "absolute";
            steps = [
              {
                color = "dark-blue";
                value = null;
              }
              {
                color = "yellow";
                value = 50;
              }
              {
                color = "orange";
                value = 300;
              }
            ];
          };
        };
      };
      targets = [
        {
          datasource = {
            type = "prometheus";
            uid = "Prometheus";
          };
          expr = "max(last_over_time(homeassistant_sensor_illuminance_lx{entity=~\"sensor.*96e8.*light.*\"}[1h]))";
          legendFormat = "Bedroom";
          refId = "A";
        }
        {
          datasource = {
            type = "prometheus";
            uid = "Prometheus";
          };
          expr = "max(last_over_time(homeassistant_sensor_illuminance_lx{entity=~\"sensor.*db2e.*light.*\"}[1h]))";
          legendFormat = "Living Room";
          refId = "B";
        }
      ];
    }
    {
      id = 7;
      type = "timeseries";
      title = "Presence History";
      gridPos = {
        x = 0;
        y = 24;
        w = 24;
        h = 8;
      };
      options = {
        tooltip = {
          mode = "multi";
        };
        legend = {
          displayMode = "list";
          placement = "bottom";
        };
      };
      fieldConfig = {
        defaults = {
          unit = "short";
          min = 0;
          max = 1;
          custom = {
            lineWidth = 2;
            fillOpacity = 20;
            drawStyle = "line";
            lineInterpolation = "stepAfter";
          };
        };
      };
      targets = [
        {
          datasource = {
            type = "prometheus";
            uid = "Prometheus";
          };
          expr = "max(last_over_time(homeassistant_binary_sensor_state{entity=~\"binary_sensor.*96e8.*presence.*\"}[1h]))";
          legendFormat = "Bedroom";
          refId = "A";
        }
        {
          datasource = {
            type = "prometheus";
            uid = "Prometheus";
          };
          expr = "max(last_over_time(homeassistant_binary_sensor_state{entity=~\"binary_sensor.*db2e.*presence.*\"}[1h]))";
          legendFormat = "Living Room";
          refId = "B";
        }
      ];
    }
    {
      id = 8;
      type = "timeseries";
      title = "Light Level History";
      gridPos = {
        x = 0;
        y = 32;
        w = 24;
        h = 8;
      };
      options = {
        tooltip = {
          mode = "multi";
        };
        legend = {
          displayMode = "list";
          placement = "bottom";
        };
      };
      fieldConfig = {
        defaults = {
          unit = "lux";
          custom = {
            lineWidth = 2;
            fillOpacity = 10;
          };
        };
      };
      targets = [
        {
          datasource = {
            type = "prometheus";
            uid = "Prometheus";
          };
          expr = "max(last_over_time(homeassistant_sensor_illuminance_lx{entity=~\"sensor.*96e8.*light.*\"}[1h]))";
          legendFormat = "Bedroom";
          refId = "A";
        }
        {
          datasource = {
            type = "prometheus";
            uid = "Prometheus";
          };
          expr = "max(last_over_time(homeassistant_sensor_illuminance_lx{entity=~\"sensor.*db2e.*light.*\"}[1h]))";
          legendFormat = "Living Room";
          refId = "B";
        }
      ];
    }
  ];
}
