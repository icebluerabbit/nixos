# dnsmasq — host configuration override for dnsmasq container
{ den, ... }:
{
  den.aspects.hosts._.server._.containers._.dnsmasq = {
    aspect-name = "hosts.server.containers.dnsmasq";
    includes = [
      den.aspects.containers._.dnsmasq
      den.aspects.infra._.clients._.container-dns
      den.aspects.hosts._.server._.networking._.vlans._.services
      den.aspects.infra._.clients._.container-loki
      den.aspects.infra._.clients._.container-prometheus
      den.aspects.infra._.clients._.container-tempo

      # VLAN networks (source of truth for DNS records)
      den.aspects.infra._.networks._.apps
      den.aspects.infra._.networks._.default
      den.aspects.infra._.networks._.dmz
      den.aspects.infra._.networks._.download
      den.aspects.infra._.networks._.game
      den.aspects.infra._.networks._.guest
      den.aspects.infra._.networks._.iot
      den.aspects.infra._.networks._.media
      den.aspects.infra._.networks._.mgmt
      den.aspects.infra._.networks._.services
      den.aspects.infra._.networks._.storage
      den.aspects.infra._.networks._.trust
      den.aspects.infra._.networks._.work

    ];

    nixos =
      { config, lib, ... }:
      {
        systemd.services."container@dnsmasq" = {
          before = [ "container-wave-foundation.target" ];
          # Weight the whole resolver container above noisy neighbours (qbt, CPUWeight=30)
          # at the machine.slice level so it wins CPU contention when the host is loaded.
          # IOWeight and MemoryMin matter as much as CPU here: the resolver was inheriting
          # the default IO weight and a reclaimable working set, so a loaded host could
          # stall or shrink its cache regardless of CPU priority.
          serviceConfig = {
            CPUWeight = 9000;
            IOWeight = 1000;
            MemoryMin = "256M";
          };
        };

        containerConfigs.dnsmasq = {

          lldpd.enable = true;
          hostBridge = "br-services";
          macAddress = config.infra.clients.container-dns.mac;
          localAddress = "${config.infra.clients.container-dns.ip}/${toString config.infra.vlans.services.subnetPrefix}";
          gateway = config.infra.vlans.services.staticAddresses.gateway;
          dnsRecords = lib.foldl' (
            acc: vlan:
            acc // lib.mapAttrs' (name: ip: lib.nameValuePair "${name}.${vlan.domain}" ip) vlan.staticAddresses
          ) { } (lib.attrValues config.infra.vlans);
          nameservers = [
            config.infra.vlans.services.staticAddresses.gateway
            "1.1.1.1"
          ];
          exporter = {
            enable = true;
            port = config.infra.clients.container-dns.ports.exporter;
          };
          logging = {
            enable = true;
            lokiAddress = "http://${config.infra.clients.container-loki.ip}:${toString config.infra.clients.container-loki.ports.default}";
            prometheusAddress = "http://${config.infra.clients.container-prometheus.ip}:${toString config.infra.clients.container-prometheus.ports.default}/api/v1/write";
            tempoAddress = "http://${config.infra.clients.container-tempo.ip}:${toString config.infra.clients.container-tempo.ports.otlp-http}/v1/traces";
            prometheusInputs = [ "dnsmasq_exporter" ];
            extraSettings.sources.dnsmasq_exporter = {
              type = "prometheus_scrape";
              endpoints = [
                "http://127.0.0.1:${toString config.infra.clients.container-dns.ports.exporter}/metrics"
              ];
              scrape_interval_secs = 30;
            };
          };
        };
      };
  };
}
