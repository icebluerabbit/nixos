# _downloads.nix — Downloads tab: torrent/soulseek clients, indexer, automation.
{
  cl,
  caddy,
  at,
  promUrl,
  ...
}:
let
  hc =
    {
      name,
      icon,
      href,
      desc,
      monitor ? null,
    }:
    {
      "${name}" = {
        inherit icon href;
        description = desc;
      }
      // (if monitor == null then { } else { siteMonitor = monitor; });
    };
in
[
  {
    "Downloads & Indexers" = [
      {
        "qBittorrent" = {
          icon = "qbittorrent.svg";
          # Card opens qui (the nicer UI); the widget pulls live stats straight
          # from qBittorrent. apps subnet is in qBittorrent's
          # authSubnetWhitelist, so auth is bypassed and the creds are dummies.
          href = caddy "qui";
          widget = {
            type = "qbittorrent";
            url = "http://${cl.container-qbittorrent.ip}:${toString cl.container-qbittorrent.ports.webui}";
            username = "admin";
            password = "admin";
            fields = [
              "leech"
              "download"
              "seed"
              "upload"
            ];
          };
        };
      }
      (hc {
        name = "qui";
        icon = "qbittorrent.svg";
        href = caddy "qui";
        monitor = at cl.container-qui.ip cl.container-qui.ports.default;
        desc = "qBittorrent web UI (qui)";
      })
      # slskd sits on the download VLAN behind the VPN gateway and isn't
      # reachable directly on :5030 from apps (ECONNREFUSED) — no status dot —
      # but it's scraped, so pull live figures from Prometheus.
      {
        "slskd" = {
          icon = "slskd.svg";
          href = caddy "slskd";
          description = "Soulseek client";
          widget = {
            type = "prometheusmetric";
            url = promUrl;
            refreshInterval = 30000;
            metrics = [
              {
                label = "Searches";
                query = "sum(slskd_search_incoming_requests_received)";
                format = {
                  type = "number";
                  options.maximumFractionDigits = 0;
                };
              }
              {
                label = "Responses";
                query = "sum(slskd_search_incoming_responses_sent)";
                format = {
                  type = "number";
                  options.maximumFractionDigits = 0;
                };
              }
              {
                label = "Children";
                query = "sum(slskd_dnet_children)";
                format = {
                  type = "number";
                  options.maximumFractionDigits = 0;
                };
              }
            ];
          };
        };
      }
      (hc {
        name = "Soularr";
        icon = "mdi-music-note-plus-#a277ff";
        href = caddy "soularr";
        monitor = at cl.container-soularr.ip cl.container-soularr.ports.webui;
        desc = "Lidarr → Soulseek automation";
      })
      {
        "Prowlarr" = {
          icon = "prowlarr.svg";
          href = caddy "prowlarr";
          widget = {
            type = "prowlarr";
            url = "http://${cl.container-prowlarr.ip}:${toString cl.container-prowlarr.ports.default}";
            key = "{{HOMEPAGE_VAR_PROWLARR_KEY}}";
            fields = [
              "numberOfGrabs"
              "numberOfQueries"
              "numberOfFailGrabs"
            ];
          };
        };
      }
      {
        "Autobrr" = {
          icon = "autobrr.svg";
          href = caddy "autobrr";
          widget = {
            type = "autobrr";
            url = "http://${cl.container-autobrr.ip}:${toString cl.container-autobrr.ports.default}";
            key = "{{HOMEPAGE_VAR_AUTOBRR_KEY}}";
            fields = [
              "approvedPushes"
              "rejectedPushes"
              "filters"
            ];
          };
        };
      }
      {
        "cross-seed" = {
          icon = "mdi-seed-#50fa7b";
          href = caddy "cross-seed";
          # `/` 404s; `/api/ping` answers 200 without a key.
          siteMonitor = "${at cl.container-cross-seed.ip cl.container-cross-seed.ports.default}/api/ping";
          description = "Automatic cross-seeding";
          widget = {
            type = "prometheusmetric";
            url = promUrl;
            refreshInterval = 60000;
            # Only cross_seed_errors_total is exported until the first
            # injection/search fires the other counters — show what's live.
            metrics = [
              {
                label = "Errors 24h";
                query = "sum(increase(cross_seed_errors_total[24h]))";
                format = {
                  type = "number";
                  options.maximumFractionDigits = 0;
                };
              }
              {
                label = "Errors/h";
                query = "sum(rate(cross_seed_errors_total[1h])) * 3600";
                format = {
                  type = "number";
                  options.maximumFractionDigits = 1;
                };
              }
            ];
          };
        };
      }
    ];
  }
]
