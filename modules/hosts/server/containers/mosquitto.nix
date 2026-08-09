# modules/hosts/server/containers/mosquitto.nix
{ den, ... }:
{
  den.aspects.hosts._.server._.containers._.mosquitto = {
    aspect-name = "hosts.server.containers.mosquitto";
    includes = [
      den.aspects.containers._.mosquitto
      den.aspects.infra._.clients._.container-mosquitto
      den.aspects.infra._.clients._.container-dns
      den.aspects.infra._.networks._.iot
      den.aspects.hosts._.server._.security
      den.aspects.hosts._.server._.networking._.vlans._.iot
      den.aspects.infra._.clients._.container-loki
      den.aspects.infra._.clients._.container-prometheus
      den.aspects.infra._.clients._.container-tempo
    ];

    nixos =
      { config, ... }:
      {
        systemd.services."container@mosquitto" = {
          before = [ "container-wave-foundation.target" ];
        };

        containerConfigs.mosquitto = {
          lldpd.enable = true;
          hostBridge = "br-iot";
          macAddress = config.infra.clients.container-mosquitto.mac;
          localAddress = "${config.infra.clients.container-mosquitto.ip}/${toString config.infra.vlans.iot.subnetPrefix}";
          gateway = config.infra.vlans.iot.staticAddresses.gateway;
          port = config.infra.clients.container-mosquitto.ports.default;
          nameservers = [
            config.infra.clients.container-dns.ip
            "1.1.1.1"
          ];
          logging = {
            enable = true;
            lokiAddress = "http://${config.infra.clients.container-loki.ip}:${toString config.infra.clients.container-loki.ports.default}";
            prometheusAddress = "http://${config.infra.clients.container-prometheus.ip}:${toString config.infra.clients.container-prometheus.ports.default}/api/v1/write";
            tempoAddress = "http://${config.infra.clients.container-tempo.ip}:${toString config.infra.clients.container-tempo.ports.otlp-http}/v1/traces";
            prometheusInputs = [ "mosquitto_metrics" ];
            extraSettings = {
              # Broker health from the $SYS tree — Vector subscribes as a local
              # anonymous client (no mosquitto exporter package exists in nixpkgs).
              sources.mosquitto_sys = {
                type = "mqtt";
                host = "127.0.0.1";
                port = config.infra.clients.container-mosquitto.ports.default;
                # `$$` escapes the literal `$` — Vector treats a lone `$` as an
                # env-var reference in its config.
                topic = "$$SYS/broker/#";
                client_id = "vector-mosquitto-metrics";
              };
              transforms = {
                # topic → metric name; payload → numeric value; drop non-numeric.
                mosquitto_parse = {
                  type = "remap";
                  inputs = [ "mosquitto_sys" ];
                  source = ''
                    topic = to_string(.topic) ?? ""
                    clean = replace(to_string(.message) ?? "", r'[^0-9.\-]', "")
                    if clean == "" { abort }
                    v, err = to_float(clean)
                    if err != null { abort }
                    .val = v
                    suffix = replace(topic, "$$SYS/broker/", "")
                    # sanitize "/" + spaces + any other non-word char → "_"
                    .metric_name = "mosquitto_" + downcase(replace(suffix, r'[^a-zA-Z0-9]+', "_"))
                  '';
                };
                mosquitto_metrics = {
                  type = "log_to_metric";
                  inputs = [ "mosquitto_parse" ];
                  metrics = [
                    {
                      type = "gauge";
                      field = "val";
                      name = "{{ metric_name }}";
                    }
                  ];
                };
              };
            };
          };
        };
      };
  };
}
