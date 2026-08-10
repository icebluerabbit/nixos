/*
  seasonpackarr.

  seasonpackarr publishes no metrics, so the Vector sidecar
  (../../seasonpackarr/_exporter.nix) derives them from two places.

  The pack outcomes come from seasonpackarr's own notifier, whose embed title is
  upstream's StatusCode enum verbatim — "successful match", "number of matches
  below threshold", "resolution did not match", and so on. That makes the
  outcome breakdown exact rather than a guess at log prose, and it is the panel
  to read first.

  The per-attribute mismatch counts come from the journal instead, because they
  are logged once per *episode compared* and the notifier only reports a
  request's final verdict. The distinction matters: `Pack outcomes` answers
  "how often did a pack fail, and why", while `Why episodes were rejected`
  answers "which attribute is doing the rejecting" — and that second panel is
  the one that says whether a fuzzyMatching flag would recover the misses.
  It is populated only because notificationLevel includes INFO.

  A pack needs smartModeThreshold (0.51) of its season already held, so most
  announces are expected to be rejected. A low match rate is normal; what is
  worth acting on is a single attribute dominating the rejection panel.
*/
let
  p = import ./_panels.nix;
  inherit (p)
    stat
    ts
    bar
    logs
    row
    target
    redAbove
    ;
in
{
  folder = "Media";
  title = "seasonpackarr";
  uid = "seasonpackarr";
  description = "seasonpackarr: season-pack match outcomes, which release attribute rejects episodes, and how much of a season is typically already held. Pack outcomes come from its notifier, attribute detail from its JSON logs.";
  editable = true;
  graphTooltip = 1;
  schemaVersion = 40;
  refresh = "1m";
  timezone = "browser";
  tags = [
    "media"
    "download"
    "seasonpackarr"
  ];

  time = {
    from = "now-7d";
    to = "now";
  };

  templating.list = [
    {
      name = "level";
      label = "Log level";
      type = "custom";
      query = "error,warn,info,debug,trace";
      multi = true;
      includeAll = true;
      allValue = ".*";
      current = {
        selected = true;
        text = [ "All" ];
        value = [ "$__all" ];
      };
      options = [ ];
    }
  ];

  panels = [
    (row {
      id = 100;
      title = "Health";
      y = 0;
    })
    (stat {
      id = 1;
      title = "Up";
      x = 0;
      y = 1;
      w = 3;
      expr = "seasonpackarr_up";
      colorMode = "background";
      textMode = "value_and_name";
      mappings = [
        {
          type = "value";
          options = {
            "0" = {
              color = "red";
              index = 0;
              text = "Down";
            };
            "1" = {
              color = "green";
              index = 1;
              text = "Up";
            };
          };
        }
      ];
      description = "/api/healthz/readiness. Goes stale rather than to 0 if the service stops answering.";
    })
    (stat {
      id = 2;
      title = "Requests";
      x = 3;
      y = 1;
      w = 3;
      expr = ''sum(increase(seasonpackarr_log_lines_total{module="processor"}[$__range])) > 0 or vector(0)'';
      decimals = 0;
      graphMode = "area";
      description = "Processor log activity — a proxy for announce volume reaching seasonpackarr.";
    })
    (stat {
      id = 3;
      title = "Packs matched";
      x = 6;
      y = 1;
      w = 3;
      expr = ''sum(increase(seasonpackarr_packs_total{outcome="successful_match"}[$__range]))'';
      decimals = 0;
      colorMode = "value";
      graphMode = "area";
    })
    (stat {
      id = 4;
      title = "Packs rejected";
      x = 9;
      y = 1;
      w = 3;
      expr = ''sum(increase(seasonpackarr_packs_total{outcome!="successful_match"}[$__range]))'';
      decimals = 0;
      description = "Expected to dominate: smartMode only accepts a pack when over half its season is already held.";
    })
    (stat {
      id = 5;
      title = "Match rate";
      x = 12;
      y = 1;
      w = 3;
      expr = ''100 * sum(increase(seasonpackarr_packs_total{outcome="successful_match"}[$__range])) / clamp_min(sum(increase(seasonpackarr_packs_total[$__range])), 1)'';
      unit = "percent";
      decimals = 1;
    })
    (stat {
      id = 6;
      title = "Season held (avg)";
      x = 15;
      y = 1;
      w = 3;
      expr = "100 * sum(increase(seasonpackarr_episodes_matched_total[$__range])) / clamp_min(sum(increase(seasonpackarr_episodes_wanted_total[$__range])), 1)";
      unit = "percent";
      decimals = 1;
      description = "Episodes already in the client as a share of those the announced packs contained. Sitting just under the 51% threshold means small changes would flip many packs to matching.";
    })
    (stat {
      id = 7;
      title = "Errors";
      x = 18;
      y = 1;
      w = 3;
      expr = ''sum(increase(seasonpackarr_log_lines_total{level="error"}[$__range]))'';
      decimals = 0;
      colorMode = "background";
      thresholds = redAbove 1;
    })
    (stat {
      id = 8;
      title = "Unauthorized";
      x = 21;
      y = 1;
      w = 3;
      expr = "sum(increase(seasonpackarr_unauthorized_total[$__range]))";
      decimals = 0;
      colorMode = "background";
      thresholds = redAbove 1;
      description = "Announces arriving without the API token — normally autobrr with a stale or missing token, which silently drops every season pack.";
    })

    (row {
      id = 101;
      title = "Pack outcomes";
      y = 5;
    })
    (bar {
      id = 10;
      title = "Pack outcomes";
      x = 0;
      y = 6;
      w = 12;
      h = 10;
      expr = "topk(15, sum by (outcome) (increase(seasonpackarr_packs_total[$__range])))";
      legendFormat = "{{outcome}}";
      description = "Upstream's StatusCode enum straight off the notification embed — no log text is matched to produce this.";
    })
    (bar {
      id = 11;
      title = "Why episodes were rejected";
      x = 12;
      y = 6;
      w = 12;
      h = 10;
      expr = "topk(10, sum by (attribute) (increase(seasonpackarr_mismatches_total[$__range])))";
      legendFormat = "{{attribute}}";
      description = "Counted per episode compared, so it sums far higher than the pack count. A single attribute dominating is the case for a fuzzyMatching flag: repack_status → skipRepackCompare, HDR → simplifyHdrCompare, source (WEB vs WEB-DL) → simplifyWebCompare.";
    })
    (ts {
      id = 12;
      title = "Matched vs rejected over time";
      x = 0;
      y = 16;
      w = 12;
      targets = [
        (target ''sum(increase(seasonpackarr_packs_total{outcome="successful_match"}[$__interval]))''
          "matched"
          "A"
        )
        (target ''sum(increase(seasonpackarr_packs_total{outcome!="successful_match"}[$__interval]))''
          "rejected"
          "B"
        )
      ];
      drawStyle = "bars";
      fillOpacity = 50;
      stacking = "normal";
      decimals = 0;
    })
    (ts {
      id = 13;
      title = "Share of season already held";
      x = 12;
      y = 16;
      w = 12;
      targets = [
        (target "seasonpackarr_match_ratio_percent" "held" "A")
        (target "51" "smartMode threshold" "B")
      ];
      unit = "percent";
      description = "Each point is one announced pack. Points above the threshold line are the ones smartMode accepts.";
    })

    (row {
      id = 102;
      title = "Logs";
      y = 24;
    })
    (ts {
      id = 20;
      title = "Log volume by level";
      x = 0;
      y = 25;
      w = 24;
      h = 6;
      targets = [
        (target ''sum by (level) (rate(seasonpackarr_log_lines_total{level=~"$level"}[$__rate_interval]))''
          "{{level}}"
          "A"
        )
      ];
      unit = "cps";
    })
    (logs {
      id = 30;
      title = "seasonpackarr log";
      x = 0;
      y = 31;
      expr = ''{host="seasonpackarr"} | json | _SYSTEMD_UNIT = "seasonpackarr.service" | line_format "{{.message}}" | json | level =~ "$level"'';
      description = "seasonpackarr logs JSON, so the message is unwrapped from the journal envelope and then parsed again to expose level/module/release as filterable fields.";
    })
  ];
}
