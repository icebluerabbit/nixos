# modules/hosts/server/containers/unifi-exporter.nix
{ den, ... }:
{
  den.aspects.hosts._.server._.containers._.unifi-exporter = {
    aspect-name = "hosts.server.containers.unifi-exporter";
    includes = [
      den.aspects.containers._.unifi-exporter
      den.aspects.infra._.clients._.container-unifi-exporter
      den.aspects.infra._.clients._.container-dns
      den.aspects.infra._.clients._.container-loki
      den.aspects.infra._.clients._.container-prometheus
      den.aspects.infra._.networks._.services
      den.aspects.hosts._.server._.security
      den.aspects.hosts._.server._.networking._.vlans._.services
      den.aspects.infra._.clients._.container-tempo
    ];

    nixos =
      { config, ... }:
      {
        systemd.services."container@unifi-exporter" = {
          after = [ "container-wave-foundation.target" ];
          before = [ "container-wave-core.target" ];
        };

        containerConfigs.unifi-exporter = {

          lldpd.enable = true;
          hostBridge = "br-services";
          macAddress = config.infra.clients.container-unifi-exporter.mac;
          port = config.infra.clients.container-unifi-exporter.ports.default;
          localAddress = "${config.infra.clients.container-unifi-exporter.ip}/${toString config.infra.vlans.services.subnetPrefix}";
          gateway = config.infra.vlans.services.staticAddresses.gateway;
          envFile = config.sops.templates."unpoller-env".path;
          syslogPort = config.infra.clients.container-unifi-exporter.ports.syslog;
          controllerUrl = "https://${config.infra.vlans.services.staticAddresses.gateway}";
          nameservers = [
            config.infra.clients.container-dns.ip
            "1.1.1.1"
          ];
          logging = {
            enable = true;
            hostType = "network";
            lokiAddress = "http://${config.infra.clients.container-loki.ip}:${toString config.infra.clients.container-loki.ports.default}";
            prometheusAddress = "http://${config.infra.clients.container-prometheus.ip}:${toString config.infra.clients.container-prometheus.ports.default}/api/v1/write";
            tempoAddress = "http://${config.infra.clients.container-tempo.ip}:${toString config.infra.clients.container-tempo.ports.otlp-http}/v1/traces";
            prometheusInputs = [ "unpoller_metrics" ];
            lokiInputs = [
              "journal"
              "unifi_syslog_standard"
              "unifi_syslog_cef"
            ];
            extraSettings = {
              sources.unpoller_metrics = {
                type = "prometheus_scrape";
                endpoints = [ "http://localhost:${toString config.containerConfigs.unifi-exporter.port}/metrics" ];
                scrape_interval_secs = 60;
              };
              sources.unifi_syslog = {
                type = "socket";
                address = "${config.infra.clients.container-unifi-exporter.ip}:${toString config.infra.clients.container-unifi-exporter.ports.syslog}";
                mode = "udp";
                decoding.codec = "bytes";
              };
              transforms.unifi_syslog_standard = {
                type = "remap";
                inputs = [ "unifi_syslog" ];
                source = ''
                  # Ensure message is treated as a string
                  .message = string!(.message)

                  # Only process standard syslog (starts with '<')
                  if !starts_with(.message, "<") {
                    abort
                  }

                  # Parse standard syslog (RFC-3164 or RFC-5424)
                  parsed, err = parse_syslog(.message)
                  if err == null {
                    .host = parsed.hostname
                    # Normalize host spaces to dashes
                    .host = replace(string!(.host), " ", "-")
                    .message = parsed.message
                  } else {
                    .host = .host || "unknown-unifi-device"
                  }

                  # Filter out the buggy U7 Pro AP wevent loop messages
                  if contains(string!(.message), "/sbin/wevent") || contains(string!(.message), "ubnt_protocol") || contains(string!(.message), "L2UF subsystem") || contains(string!(.message), "Successfully initialized VAP tracking") {
                    abort
                  }
                '';
              };
              transforms.unifi_syslog_cef = {
                type = "remap";
                inputs = [ "unifi_syslog" ];
                source = ''
                  # Ensure message is treated as a string
                  .message = string!(.message)

                  # Only process CEF (does not start with '<')
                  if starts_with(.message, "<") {
                    abort
                  }

                  # Parse out the ISO UTC timestamp, device name, and CEF payload
                  parsed, err = parse_regex(.message, r'^(?P<syslog_time>[A-Z][a-z]{2}\s+\d+\s+\d{2}:\d{2}:\d{2})\s+(?P<iso_time>\S+)\s+(?P<device_name>.+?)\s+(?P<payload>CEF:\d+\|.*)$')
                  if err == null {
                    .host = parsed.device_name
                    # Normalize host spaces to dashes
                    .host = replace(string!(.host), " ", "-")
                    .message = parsed.payload

                    # Parse the standard CEF structured data
                    cef, cef_err = parse_cef(string!(parsed.payload))
                    if cef_err == null {
                      . = merge(., cef)
                    }

                    # Parse the ISO timestamp which explicitly has 'Z' (UTC)
                    ts, ts_err = parse_timestamp(parsed.iso_time, "%Y-%m-%dT%H:%M:%S%.f%Z")
                    if ts_err == null {
                      .timestamp = ts
                    }
                  } else {
                    .host = .host || "unknown-unifi-device"
                  }
                '';
              };
            };
          };
        };
      };
  };
}
