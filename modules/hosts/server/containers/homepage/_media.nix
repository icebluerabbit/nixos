# _media.nix — Media tab: Jellyfin, the *arr library managers, books, and the
# post-processing/utility services.
{
  cl,
  caddy,
  at,
  promUrl,
  ...
}:
let
  # Health-dot card for a service with no native widget. `href` opens the nice
  # Caddy-proxied UI; `monitor` is the DIRECT http endpoint (Caddy 308-redirects
  # to https, which Homepage's http client refuses to follow). Omit `monitor`
  # for services that don't answer a direct GET.
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
    "Media" = [
      {
        "Jellyfin" = {
          icon = "jellyfin.svg";
          href = caddy "jellyfin";
          widget = {
            type = "jellyfin";
            url = "http://${cl.container-jellyfin.ip}:${toString cl.container-jellyfin.ports.http}";
            key = "{{HOMEPAGE_VAR_JELLYFIN_KEY}}";
            enableBlocks = true;
            enableNowPlaying = true;
          };
        };
      }
      {
        "Sonarr" = {
          icon = "sonarr.svg";
          href = caddy "sonarr";
          widget = {
            type = "sonarr";
            url = "http://${cl.container-sonarr.ip}:${toString cl.container-sonarr.ports.default}";
            key = "{{HOMEPAGE_VAR_SONARR_KEY}}";
            enableQueue = true;
            fields = [
              "wanted"
              "queued"
              "series"
            ];
          };
        };
      }
      {
        "Radarr" = {
          icon = "radarr.svg";
          href = caddy "radarr";
          widget = {
            type = "radarr";
            url = "http://${cl.container-radarr.ip}:${toString cl.container-radarr.ports.default}";
            key = "{{HOMEPAGE_VAR_RADARR_KEY}}";
            enableQueue = true;
            fields = [
              "wanted"
              "missing"
              "queued"
              "movies"
            ];
          };
        };
      }
      # Plain health-dot card: Homepage has no stash widget type.
      (hc {
        name = "Stash";
        icon = "stash.svg";
        href = caddy "stash";
        monitor = at cl.container-stash.ip cl.container-stash.ports.default;
        desc = "Adult library organizer & player";
      })
      {
        "Lidarr" = {
          icon = "lidarr.svg";
          href = caddy "lidarr";
          widget = {
            type = "lidarr";
            url = "http://${cl.container-lidarr.ip}:${toString cl.container-lidarr.ports.default}";
            key = "{{HOMEPAGE_VAR_LIDARR_KEY}}";
            fields = [
              "wanted"
              "queued"
              "artists"
            ];
          };
        };
      }
      {
        "Readarr" = {
          icon = "readarr.svg";
          href = caddy "readarr";
          widget = {
            type = "readarr";
            url = "http://${cl.container-readarr.ip}:${toString cl.container-readarr.ports.default}";
            key = "{{HOMEPAGE_VAR_READARR_KEY}}";
            fields = [
              "wanted"
              "queued"
              "books"
            ];
          };
        };
      }
      {
        "Bazarr" = {
          icon = "bazarr.svg";
          href = caddy "bazarr";
          widget = {
            type = "bazarr";
            url = "http://${cl.container-bazarr.ip}:${toString cl.container-bazarr.ports.default}";
            key = "{{HOMEPAGE_VAR_BAZARR_KEY}}";
            fields = [
              "missingEpisodes"
              "missingMovies"
            ];
          };
        };
      }
      (hc {
        name = "Beets";
        icon = "mdi-music-box-multiple-#bd93f9";
        href = caddy "beets";
        monitor = at cl.container-beets.ip cl.container-beets.ports.default;
        desc = "CD-rip tagger & library organizer";
      })
      (hc {
        name = "Calibre-Web";
        icon = "calibre-web.svg";
        href = caddy "calibre-web";
        monitor = at cl.container-calibre-web.ip cl.container-calibre-web.ports.default;
        desc = "eBook library & reader";
      })
      {
        "Bindery" = {
          icon = "mdi-book-open-page-variant-#8be9fd";
          href = caddy "bindery";
          siteMonitor = at cl.container-bindery.ip cl.container-bindery.ports.default;
          description = "Book & audiobook automation";
          # No native Homepage widget. Bindery's own /metrics carries HTTP and
          # scheduler series only — no library or queue counts — so these are the
          # useful KPIs available.
          widget = {
            type = "prometheusmetric";
            url = promUrl;
            refreshInterval = 60000;
            metrics = [
              {
                label = "Jobs 24h";
                query = "sum(increase(bindery_scheduler_runs_total[24h]))";
                format = {
                  type = "number";
                  options.maximumFractionDigits = 0;
                };
              }
              {
                label = "5xx 24h";
                query = ''sum(increase(bindery_http_requests_total{status=~"5.."}[24h]))'';
                format = {
                  type = "number";
                  options.maximumFractionDigits = 0;
                };
              }
            ];
          };
        };
      }
      {
        "Komga" = {
          icon = "komga.svg";
          href = caddy "komga";
          siteMonitor = at cl.container-komga.ip cl.container-komga.ports.default;
          description = "Comic & manga server";
          # Komga's widget authenticates with the admin account rather than an API
          # key — which is what we have in sops. The runtime-minted key lives
          # inside the Komga container and Homepage could not read it anyway.
          widget = {
            type = "komga";
            url = at cl.container-komga.ip cl.container-komga.ports.default;
            username = "{{HOMEPAGE_VAR_KOMGA_USERNAME}}";
            password = "{{HOMEPAGE_VAR_KOMGA_PASSWORD}}";
          };
        };
      }
      # Audiobookshelf's widget needs an API token, but that token is minted at
      # runtime inside the ABS container and Homepage runs elsewhere — containers
      # talk over IP here, not shared bind mounts. Health dot until that changes.
      (hc {
        name = "Audiobookshelf";
        icon = "audiobookshelf.svg";
        href = caddy "audiobookshelf";
        monitor = at cl.container-audiobookshelf.ip cl.container-audiobookshelf.ports.default;
        desc = "Audiobook & podcast server";
      })
      {
        "Youtarr" = {
          icon = "youtube.svg";
          href = caddy "youtarr";
          siteMonitor = at cl.container-youtarr.ip cl.container-youtarr.ports.http;
          description = "YouTube DVR & automation";
          widget = {
            type = "prometheusmetric";
            url = promUrl;
            refreshInterval = 15000;
            metrics = [
              {
                label = "Done 24h";
                query = "sum(increase(youtarr_downloads_completed_total[24h]))";
                format = {
                  type = "number";
                  options.maximumFractionDigits = 0;
                };
              }
              {
                label = "Speed";
                query = "sum(youtarr_download_speed_bytes_per_second)";
                format = {
                  type = "bytes";
                  suffix = "/s";
                };
              }
              {
                label = "Progress";
                query = "max(youtarr_download_percent)";
                format = {
                  type = "number";
                  suffix = "%";
                  options.maximumFractionDigits = 0;
                };
              }
            ];
          };
        };
      }
      # Kiwix doesn't answer a direct GET on :8080 from the apps VLAN — link only.
      (hc {
        name = "Kiwix";
        icon = "kiwix.svg";
        href = caddy "kiwix";
        desc = "Offline Wikipedia & web archives";
      })
      {
        "Unpackerr" = {
          icon = "mdi-package-variant-#f1fa8c";
          href = caddy "grafana";
          description = "Archive extraction for the *arr stack";
          widget = {
            type = "prometheusmetric";
            url = promUrl;
            refreshInterval = 15000;
            metrics = [
              {
                label = "Extracting";
                query = "sum(unpackerr_queue_extracting)";
                format = {
                  type = "number";
                  options.maximumFractionDigits = 0;
                };
              }
              {
                label = "Queued";
                query = "sum(unpackerr_queue_queued)";
                format = {
                  type = "number";
                  options.maximumFractionDigits = 0;
                };
              }
              {
                label = "Extracted";
                query = "sum(unpackerr_queue_extracted)";
                format = {
                  type = "number";
                  options.maximumFractionDigits = 0;
                };
              }
            ];
          };
        };
      }
      {
        "FlareSolverr" = {
          icon = "mdi-shield-search-#8be9fd";
          href = at cl.container-flaresolverr.ip cl.container-flaresolverr.ports.default;
          siteMonitor = at cl.container-flaresolverr.ip cl.container-flaresolverr.ports.default;
          description = "Cloudflare challenge solver";
          widget = {
            type = "prometheusmetric";
            url = promUrl;
            refreshInterval = 30000;
            metrics = [
              {
                label = "Reqs/h";
                query = "sum(increase(flaresolverr_requests_total[1h]))";
                format = {
                  type = "number";
                  options.maximumFractionDigits = 0;
                };
              }
              {
                label = "Resp";
                query = "avg_over_time(flaresolverr_response_seconds[5m])";
                format = {
                  type = "number";
                  suffix = "s";
                  options.maximumFractionDigits = 2;
                };
              }
              {
                label = "Errors 24h";
                query = "sum(increase(flaresolverr_errors_total[24h]))";
                format = {
                  type = "number";
                  options.maximumFractionDigits = 0;
                };
              }
            ];
          };
        };
      }
    ];
  }
]
