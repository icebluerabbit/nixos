# modules/hosts/server/containers/grafana.nix
{ den, ... }:
{
  den.aspects.hosts._.server._.containers._.grafana = {
    aspect-name = "hosts.server.containers.grafana";
    includes = [
      den.aspects.containers._.grafana
      den.aspects.infra._.clients._.container-grafana
      den.aspects.infra._.clients._.container-loki
      den.aspects.infra._.clients._.container-prometheus
      den.aspects.infra._.clients._.container-mosquitto
      den.aspects.infra._.clients._.container-dns
      den.aspects.infra._.networks._.services
      den.aspects.hosts._.server._.networking._.vlans._.services
      den.aspects.containers._.postgresql
      den.aspects.infra._.clients._.container-tempo
    ];

    nixos =
      {
        config,
        lib,
        pkgs,
        ...
      }:
      let
        cfg = config.containerConfigs.grafana;

        # Everything Grafana provisions — datasources, dashboards, alert rules — is one
        # file per artefact, listed exactly once below. Each file declares its own
        # placement: a dashboard carries `uid` and `folder`, an alert file carries the
        # `group` its rules belong to. Nothing here restates what the file already says,
        # so there is no second list to fall out of sync with the first.

        # Dashboard attrset → provisioning provider. `folder` is stripped before
        # serialising — it isn't a grafana dashboard field — and `uid` doubles as the
        # provider name, since it is already the dashboard's identity.
        mkProvider = d: {
          name = d.uid;
          folder = d.folder or "";
          options.path = pkgs.writeTextDir "${d.uid}.json" (builtins.toJSON (removeAttrs d [ "folder" ]));
        };

        # The landing dashboard is referenced twice on purpose: once as a provisioned
        # dashboard like any other, and once by path as Grafana's default home.
        homeDashboard = import ./dashboards/_home.nix;

        # Alert rule files are `{ group, rules }`; a group becomes one Grafana rule
        # group, evaluated on one interval, in one folder.
        alertLib = import ./alerts/_lib.nix { };
        mkGroups =
          files:
          lib.mapAttrsToList (name: entries: {
            orgId = 1;
            inherit name;
            folder = "Alerts";
            interval = "1m";
            rules = lib.concatMap (e: e.rules) entries;
          }) (lib.groupBy (e: e.group) (map (f: import f alertLib) files));
      in
      {
        containerConfigs.grafana = {

          lldpd.enable = true;
          port = config.infra.clients.container-grafana.ports.default;
          enableTelemetry = false;
          hostBridge = "br-services";
          macAddress = config.infra.clients.container-grafana.mac;
          localAddress = "${config.infra.clients.container-grafana.ip}/${toString config.infra.vlans.services.subnetPrefix}";
          gateway = config.infra.vlans.services.staticAddresses.gateway;
          lokiAddress = "http://${config.infra.clients.container-loki.ip}:${toString config.infra.clients.container-loki.ports.default}";
          prometheusAddress = "http://${config.infra.clients.container-prometheus.ip}:${toString config.infra.clients.container-prometheus.ports.default}";
          tempoAddress = "http://${config.infra.clients.container-tempo.ip}:${toString config.infra.clients.container-tempo.ports.default}";
          envFile = config.sops.templates."grafana-env".path;
          bindMounts = {
            "/var/lib/grafana" = {
              hostPath = "/var/lib/appdata/grafana";
              isReadOnly = false;
            };
          };
          nameservers = [
            config.infra.clients.container-dns.ip
            "1.1.1.1"
          ];
          dbHost = config.infra.clients.container-postgres.ip;
          dbPort = config.infra.clients.container-postgres.ports.default;

          settings = {
            dashboards.default_home_dashboard_path = "${pkgs.writeText "home.json" (
              builtins.toJSON homeDashboard
            )}";
            "auth.basic".enabled = true;
            "auth.proxy" = {
              enabled = true;
              header_name = "Remote-User";
              headers = "Name:Remote-Name";
              auto_sign_up = true;
              sync_ttl = 720;
            };
            "auth.anonymous".enabled = false;
          };

          provision = {
            # Datasources — one file per source in ./datasources/. uids are pinned to the
            # existing postgres uids so provisioning updates in place; alert rules resolve
            # datasources by real uid, never by name.
            datasources.settings.datasources = map (f: import f { inherit config; }) [
              ./datasources/_loki.nix
              ./datasources/_prometheus.nix
              ./datasources/_tempo.nix
            ];

            # Dashboards — one file per dashboard in ./dashboards/. Each carries its own
            # uid and folder, so this list is only "which dashboards exist".
            dashboards.settings.providers = map mkProvider [
              homeDashboard
              (import ./dashboards/_authelia.nix)
              (import ./dashboards/_autobrr.nix)
              (import ./dashboards/_bazarr.nix)
              (import ./dashboards/_claude.nix)
              (import ./dashboards/_container_logs.nix)
              (import ./dashboards/_container_metrics.nix)
              (import ./dashboards/_cross-seed.nix)
              (import ./dashboards/_dnsmasq.nix)
              (import ./dashboards/_flaresolverr.nix)
              (import ./dashboards/_home_sensors.nix)
              (import ./dashboards/_host_logs.nix)
              (import ./dashboards/_host_metrics.nix)
              (import ./dashboards/_jellyfin.nix)
              (import ./dashboards/_bindery.nix)
              (import ./dashboards/_komga.nix)
              (import ./dashboards/_audiobookshelf.nix)
              (import ./dashboards/_lidarr.nix)
              (import ./dashboards/_mam.nix)
              (import ./dashboards/_managarr.nix)
              (import ./dashboards/_mosquitto.nix)
              (import ./dashboards/_network_logs.nix)
              (import ./dashboards/_network_metrics.nix)
              (import ./dashboards/_nfs.nix)
              (import ./dashboards/_opencode.nix)
              (import ./dashboards/_pi-hole.nix)
              (import ./dashboards/_postgres.nix)
              (import ./dashboards/_prowlarr.nix)
              (import ./dashboards/_qbittorrent.nix)
              (import ./dashboards/_radarr.nix)
              (import ./dashboards/_seasonpackarr.nix)
              (import ./dashboards/_smart.nix)
              (import ./dashboards/_sonarr.nix)
              (import ./dashboards/_soularr.nix)
              (import ./dashboards/_stashtrap.nix)
              (import ./dashboards/_soulseek.nix)
              (import ./dashboards/_unpackerr.nix)
              (import ./dashboards/_vpn_gw.nix)
              (import ./dashboards/_youtarr.nix)
              (import ./dashboards/_zwave.nix)
            ];

            alerting = {
              # The one contact point. Grafana POSTs alert batches to the grafana Vector
              # sidecar on loopback (see logging.extraSettings below); the sidecar explodes
              # alerts[] into one MQTT message per alert on
              # homelab/notify/grafana/alerts/<alertname>, and HA maps that topic to a
              # Matrix room. Rooms are the notification layer's business — a rule never
              # names one, and neither does Grafana.
              contactPoints.settings = {
                apiVersion = 1;
                contactPoints = [
                  {
                    orgId = 1;
                    name = "matrix";
                    receivers = [
                      {
                        uid = "matrix";
                        type = "webhook";
                        settings = {
                          url = "http://127.0.0.1:9001/";
                          httpMethod = "POST";
                        };
                      }
                    ];
                  }
                ];
              };

              # Notification policy. Still load-bearing after the move to MQTT: the policy
              # decides *when* Grafana speaks, which is upstream of how the message travels.
              # It groups instances into one POST, waits out a flap before saying anything,
              # and rations repeats of a fault that is still firing. Only the routing half
              # died — with a single receiver there are no nested routes to write.
              #
              # group_by carries `host` so a fleet-wide event (an NFS drop, a Vector stall)
              # arrives as one notification per affected target instead of collapsing into
              # a single opaque line.
              policies.settings = {
                apiVersion = 1;
                policies = [
                  {
                    orgId = 1;
                    receiver = "matrix";
                    group_by = [
                      "alertname"
                      "host"
                    ];
                    group_wait = "30s";
                    group_interval = "5m";
                    repeat_interval = "4h";
                  }
                ];
              };

              # Alert rules — one file per alert in ./alerts/. Each declares the group it
              # belongs to, so this list is only "which alerts exist".
              rules.settings = {
                apiVersion = 1;
                # One-shot tombstone: Grafana file provisioning never deletes an
                # orphaned rule when its source file is removed, so the rule from
                # the deleted _tunnel_down.nix lingered (still provenance=file,
                # undeletable via API/UI). deleteRules purges it on reload.
                # Idempotent for a missing uid — safe to keep or drop once gone.
                deleteRules = [
                  {
                    orgId = 1;
                    uid = "vpn_tunnel_down";
                  }
                  {
                    orgId = 1;
                    uid = "flac_suspect";
                  }
                ];
                groups = mkGroups [
                  ./alerts/_alerting_broken.nix
                  ./alerts/_arr_health_issue.nix
                  ./alerts/_authelia_auth_failures.nix
                  ./alerts/_autobrr_irc_down.nix
                  ./alerts/_backup_stale.nix
                  ./alerts/_bazarr_backend_down.nix
                  ./alerts/_bazarr_providers_throttled.nix
                  ./alerts/_bond_degraded.nix
                  ./alerts/_btrfs_allocation_low.nix
                  ./alerts/_btrfs_device_errors.nix
                  ./alerts/_caddy_reload_failed.nix
                  ./alerts/_cross_seed_action_failed.nix
                  ./alerts/_dhcp_pool_low.nix
                  ./alerts/_disk_fill_predicted.nix
                  ./alerts/_dns_upstream_failures.nix
                  ./alerts/_ha_battery_low.nix
                  ./alerts/_ha_device_unavailable.nix
                  ./alerts/_io_stalled.nix
                  ./alerts/_isp_latency.nix
                  ./alerts/_jellyfin_library_shrank.nix
                  ./alerts/_jellyfin_transcode_load.nix
                  ./alerts/_journal_error_burst.nix
                  ./alerts/_load_high.nix
                  ./alerts/_loki_wal_disk_full.nix
                  ./alerts/_media_mount_missing.nix
                  ./alerts/_media_share_low.nix
                  ./alerts/_memory_stalled.nix
                  ./alerts/_nfs_retransmissions.nix
                  ./alerts/_no_healthy_tunnels.nix
                  ./alerts/_oom_kill.nix
                  ./alerts/_pihole_down.nix
                  ./alerts/_pihole_gravity_empty.nix
                  ./alerts/_pihole_internal_split_broken.nix
                  ./alerts/_pihole_upstream_slow.nix
                  ./alerts/_postgres_connections_high.nix
                  ./alerts/_postgres_deadlocks.nix
                  ./alerts/_postgres_down.nix
                  ./alerts/_prowlarr_indexer_failures.nix
                  ./alerts/_qbt_disk_low.nix
                  ./alerts/_qbt_disk_queue.nix
                  ./alerts/_qbt_vpn_disconnected.nix
                  ./alerts/_root_disk_low.nix
                  ./alerts/_run_tmpfs_low.nix
                  ./alerts/_smart_critical_warning.nix
                  ./alerts/_smart_spare_low.nix
                  ./alerts/_smart_wearout.nix
                  ./alerts/_swap_thrashing.nix
                  ./alerts/_system_degraded.nix
                  ./alerts/_systemd_unit_failed.nix
                  ./alerts/_target_down.nix
                  ./alerts/_tunnel_chronically_dead.nix
                  ./alerts/_tunnel_flapping.nix
                  ./alerts/_unifi_controller_down.nix
                  ./alerts/_unifi_device_hot.nix
                  ./alerts/_unifi_port_errors.nix
                  ./alerts/_unit_flapping.nix
                  ./alerts/_vector_component_errors.nix
                  ./alerts/_vector_dropping_events.nix
                  ./alerts/_vpn_pool_capacity.nix
                  ./alerts/_vpn_port_forward_lost.nix
                  ./alerts/_slskd_port_forward_lost.nix
                  ./alerts/_wan_down.nix
                  ./alerts/_web_service_down.nix
                  ./alerts/_wifi_channel_saturated.nix
                  ./alerts/_zigbee_bridge_silent.nix
                  ./alerts/_zigbee_mqtt_disconnected.nix
                ];
              };
            };
          };
          logging = {
            enable = true;
            lokiAddress = "http://${config.infra.clients.container-loki.ip}:${toString config.infra.clients.container-loki.ports.default}";
            prometheusAddress = "http://${config.infra.clients.container-prometheus.ip}:${toString config.infra.clients.container-prometheus.ports.default}/api/v1/write";
            tempoAddress = "http://${config.infra.clients.container-tempo.ip}:${toString config.infra.clients.container-tempo.ports.otlp-http}/v1/traces";
            prometheusInputs = [ "grafana_metrics" ];
            # Each alert evaluation logs two info lines per datasource query (~3000/day
            # for a handful of rules). These are also the lines that quote the alert
            # query verbatim — the self-match loop the oom_kill / cross_seed_action_failed
            # rules are written around. Alert state changes come from
            # ngalert.state.manager and keep shipping.
            journalDropPatterns = [ "uname=grafana_scheduler" ];
            extraSettings = {
              sources.grafana_metrics = {
                type = "prometheus_scrape";
                endpoints = [ "http://localhost:${toString config.containerConfigs.grafana.port}/metrics" ];
                scrape_interval_secs = 60;
              };
              # Grafana's alerting webhook contact points POST here on loopback; each
              # POST batches alerts[] — the remap explodes it into one event per alert
              # and the sink routes each to homelab/notify/grafana/alerts/<alertname>.
              sources.webhook_in = {
                type = "http_server";
                address = "127.0.0.1:9001";
                method = "POST";
                path = "/";
                decoding.codec = "json"; # parse the grafana payload to read .alerts
                framing.method = "bytes"; # decode the whole request body as one doc, not per line
              };
              transforms.grafana_alerts = {
                type = "remap";
                inputs = [ "webhook_in" ];
                source = ''
                  alerts = array(.alerts) ?? []
                  # assigning an array to `.` fans one input event out into one per element
                  . = map_values(alerts) -> |alert| {
                    name = downcase(replace(to_string(alert.labels.alertname) ?? "unknown", r'[^a-zA-Z0-9]+', "_"))
                    { "alertname": name, "message": encode_json(alert) }
                  }
                '';
              };
              sinks.webhook_mqtt = {
                type = "mqtt";
                inputs = [ "grafana_alerts" ];
                host = config.infra.clients.container-mosquitto.ip;
                port = config.infra.clients.container-mosquitto.ports.default;
                client_id = "vector-grafana"; # unique per container → no broker client-id collision
                topic = "homelab/notify/grafana/alerts/{{ alertname }}"; # per-alert, slugified
                quality_of_service = "atleastonce";
                retain = false;
                encoding.codec = "text"; # emit .message = the single-alert JSON
              };
            };
          };
        };

        systemd.tmpfiles.rules = [
          "d /var/lib/appdata/grafana 0770 ${toString cfg.uid} ${toString cfg.gid} - -"
          "Z /var/lib/appdata/grafana 0770 ${toString cfg.uid} ${toString cfg.gid} - -"
        ];

        systemd.services."container@grafana" = {
          before = [ "container-wave-core.target" ];
          wants = [
            "container@loki.service"
            "container@postgresql.service"
          ];
          after = [
            "container@loki.service"
            "container@postgresql.service"
            "container-wave-foundation.target"
          ];
        };
      };
  };
}
