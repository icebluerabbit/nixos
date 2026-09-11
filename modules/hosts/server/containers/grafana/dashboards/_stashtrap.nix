/*
  stashtrap.

  stashtrap is autobrr's external filter for BitPorn: autobrr asks it about each
  release, it records the announce and answers with a status code, and autobrr
  performs the grab itself. Every series here comes from stashtrap's own
  Prometheus endpoint, scraped by the Vector sidecar on loopback.

  A release is accepted when it names a followed performer or studio, and
  rejected otherwise — so "Accepted" is the grab count, and the followed
  entities are what drives it.

  Things to know when reading these panels:

    * `no-favourite` is the normal rejection: the release was understood and
      features nobody followed. `record-only` means something else entirely —
      the catalogue was empty, so the question could not be asked. A rising
      `record-only` is an alarm; a rising `no-favourite` is a Tuesday.
    * Accept rate is low by design. Roughly 3% of BitPorn's output names one of
      the followed entities, so single digits per day is the expected shape and
      a sudden jump means the matcher started believing things it should not.
    * `stashtrap_reading_score` is the evidence behind each reading. Zero means
      nothing in the name was recognised; the mass above ~6 is readings where a
      studio and a performer, or a scene, agreed with each other.
    * `stashtrap_announce_files` is only observed when the announce carried a
      .torrent. A count of 1 is a single scene; the tail above 10 is packs,
      which are the releases one announce cannot describe on its own.
    * `stashtrap_announce_categories_total` counts category assignments, not
      announces — BitPorn puts two on every release, the constant standard
      6000/XXX bucket plus one of its own — so it will always read about twice
      `stashtrap_announces_total`. The category panel filters 6000 out.
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
    ;
in
{
  folder = "Media";
  title = "stashtrap";
  uid = "stashtrap";
  description = "stashtrap: announces received from autobrr, the decision taken on each, release sizes and pack shape, and the service's own logs.";
  editable = true;
  graphTooltip = 1;
  schemaVersion = 40;
  refresh = "1m";
  timezone = "browser";
  tags = [
    "media"
    "download"
    "stashtrap"
  ];

  time = {
    from = "now-7d";
    to = "now";
  };

  panels = [
    (row {
      id = 100;
      title = "Health";
      y = 0;
    })
    (stat {
      id = 1;
      title = "Announces (24h)";
      x = 0;
      y = 1;
      w = 4;
      expr = "sum(increase(stashtrap_announces_total[24h]))";
      decimals = 0;
      description = "Releases autobrr asked about in the last day. Zero means the feed produced nothing new, not that the service is down.";
    })
    (stat {
      id = 2;
      title = "Grabbed (24h)";
      x = 4;
      y = 1;
      w = 4;
      expr = ''sum(increase(stashtrap_announces_total{decision="accept"}[24h])) or vector(0)'';
      decimals = 0;
      colorMode = "background";
      description = "Releases accepted, which is one grab each: autobrr adds every 250 to qBittorrent under the xxx category.";
    })
    (stat {
      id = 3;
      title = "Accept rate (24h)";
      x = 8;
      y = 1;
      w = 4;
      expr = ''100 * (sum(increase(stashtrap_announces_total{decision="accept"}[24h])) or vector(0)) / clamp_min(sum(increase(stashtrap_announces_total[24h])), 1)'';
      unit = "percent";
      decimals = 1;
      description = "About 3% is the measured rate over the recorded corpus. A sharp rise means the matcher started believing something it should not, and is worth reading the accepted names for.";
    })
    (stat {
      id = 4;
      title = "Since last grab";
      x = 12;
      y = 1;
      w = 4;
      expr = ''time() - max(timestamp(stashtrap_announces_total{decision="accept"} > 0)) or vector(0)'';
      unit = "s";
      decimals = 0;
      description = "How long since anything was taken. Days here is normal at a 3% rate; weeks means either the feed stopped or matching broke.";
    })
    (stat {
      id = 5;
      title = "Decide p99";
      x = 16;
      y = 1;
      w = 4;
      expr = "histogram_quantile(0.99, sum by (le) (rate(stashtrap_decide_seconds_bucket[1h])))";
      unit = "s";
      decimals = 3;
      description = "Parse, match, decide and record one announce. autobrr blocks on this, so a slow answer delays its grab. Matching costs a few hundred milliseconds; seconds means the catalogue queries are struggling.";
    })
    (stat {
      id = 6;
      title = "Torrents parsed (24h)";
      x = 20;
      y = 1;
      w = 4;
      expr = "sum(increase(stashtrap_announce_files_count[24h])) or vector(0)";
      decimals = 0;
      description = "Announces that carried a .torrent. A gap against the announce count means the filter template stopped sending one.";
    })

    (row {
      id = 101;
      title = "Decisions";
      y = 5;
    })
    (ts {
      id = 10;
      title = "Announces per hour";
      x = 0;
      y = 6;
      w = 12;
      unit = "short";
      description = "Announce rate by outcome. The accept line is grabs; the reject line is the tracker's upload cadence.";
      targets = [
        (target "sum by (decision) (increase(stashtrap_announces_total[1h]))" "{{decision}}" "A")
      ];
    })
    (bar {
      id = 11;
      title = "Decisions by reason (24h)";
      x = 12;
      y = 6;
      w = 12;
      expr = "sum by (reason) (increase(stashtrap_announces_total[24h]))";
      legendFormat = "{{reason}}";
      description = "`no-favourite` is the ordinary reject: understood, features nobody followed. `record-only` is not — it means the catalogue was empty and the question could not be asked, so a sync has failed or never ran.";
    })
    (bar {
      id = 12;
      title = "Announces by indexer (24h)";
      x = 0;
      y = 14;
      w = 12;
      expr = "sum by (indexer) (increase(stashtrap_announces_total[24h]))";
      legendFormat = "{{indexer}}";
      description = "Only BitPorn is wired to stashtrap today; another name here means a second filter was pointed at it.";
    })
    (bar {
      id = 13;
      title = "Categories (24h)";
      x = 12;
      y = 14;
      w = 12;
      # 6000 is excluded on purpose: it is the standard Torznab XXX bucket and
      # rides on every single release, so leaving it in produces one bar equal
      # to the announce total that dwarfs everything with actual signal.
      expr = ''sum by (category) (increase(stashtrap_announce_categories_total{category_id!="6000"}[24h]))'';
      legendFormat = "{{category}}";
      description = "BitPorn's own category on each release — its only taxonomy, since the feed carries no tags. Names come from Prowlarr's caps, which currently end each one with a stray double quote.";
    })

    (row {
      id = 105;
      title = "Matching";
      y = 22;
    })
    (bar {
      id = 50;
      title = "Followed entities named (7d)";
      x = 0;
      y = 23;
      w = 12;
      h = 9;
      expr = "sum by (name) (increase(stashtrap_favourite_hits_total[7d]))";
      legendFormat = "{{name}}";
      description = "Who the grabs were for. An empty panel with a healthy announce rate means nothing followed has been released this week — check a name here against the accepted releases below before assuming the matcher is broken.";
    })
    (ts {
      id = 51;
      title = "Entities recognised per hour";
      x = 12;
      y = 23;
      w = 12;
      h = 9;
      unit = "short";
      description = "What the catalogue recognised in release names, followed or not. Studios dominate because every release leads with one; a collapse to zero across all kinds means the catalogue emptied or the alias tables were pruned too hard.";
      targets = [
        (target "sum by (kind) (increase(stashtrap_recognised_total[1h]))" "{{kind}}" "A")
      ];
    })
    (ts {
      id = 52;
      title = "Reading score";
      x = 0;
      y = 32;
      w = 12;
      unit = "short";
      description = "Evidence behind each reading. p50 near zero is expected — most releases name nobody followed — while the p95 line is where recognition actually happens. A p95 falling toward zero is matching degrading before any accept count changes.";
      targets = [
        (target "histogram_quantile(0.5, sum by (le) (rate(stashtrap_reading_score_bucket[6h])))" "p50" "A")
        (target "histogram_quantile(0.95, sum by (le) (rate(stashtrap_reading_score_bucket[6h])))" "p95"
          "B"
        )
        (target "histogram_quantile(0.99, sum by (le) (rate(stashtrap_reading_score_bucket[6h])))" "p99"
          "C"
        )
      ];
    })
    (ts {
      id = 53;
      title = "Scenes identified per hour";
      x = 12;
      y = 32;
      w = 12;
      unit = "short";
      description = "Releases matched to a known scene, split by whether the parts of the reading agreed with each other. Uncorroborated identifications resting on a title alone are the ones that have historically been wrong, so a rising `false` line deserves a look at the names.";
      targets = [
        (target "sum by (corroborated) (increase(stashtrap_scenes_identified_total[1h]))"
          "corroborated={{corroborated}}"
          "A"
        )
      ];
    })

    (row {
      id = 104;
      title = "Catalogue";
      y = 40;
    })
    (stat {
      id = 40;
      title = "Performers";
      x = 0;
      y = 41;
      w = 4;
      expr = ''stashtrap_catalogue_entities{entity="performers"}'';
      decimals = 0;
      description = "Favourited performers stored locally. This is the watchlist matching is constrained to.";
    })
    (stat {
      id = 41;
      title = "Studios";
      x = 4;
      y = 41;
      w = 4;
      expr = ''stashtrap_catalogue_entities{entity="studios"}'';
      decimals = 0;
    })
    (stat {
      id = 42;
      title = "Scenes";
      x = 8;
      y = 41;
      w = 4;
      expr = ''stashtrap_catalogue_entities{entity="scenes"}'';
      decimals = 0;
      description = "Distinct scenes stored. Lower than the fetched count: a scene credited to a followed performer at a followed studio arrives twice and is stored once.";
    })
    (stat {
      id = 43;
      title = "Last sync";
      x = 12;
      y = 41;
      w = 4;
      expr = "time() - stashtrap_sync_last_success_timestamp_seconds";
      unit = "s";
      decimals = 0;
      description = "Age of the last crawl that finished without error. The timer runs daily, so more than about a day here means the crawl is failing or the timer is not firing.";
      # Amber at two days, red at three: one missed run is a blip, three is
      # a catalogue drifting away from what the account actually follows.
      thresholds = [
        {
          color = "green";
          value = null;
        }
        {
          color = "yellow";
          value = 172800;
        }
        {
          color = "red";
          value = 259200;
        }
      ];
    })
    (stat {
      id = 44;
      title = "Sync duration";
      x = 16;
      y = 41;
      w = 4;
      expr = "stashtrap_sync_duration_seconds";
      unit = "s";
      decimals = 0;
      description = "How long the last crawl took. It is one request per page at a one-second floor, plus a scene query per followed entity, so this grows with the watchlist.";
    })
    (stat {
      id = 45;
      title = "Sync failures";
      x = 20;
      y = 41;
      w = 4;
      expr = "sum(increase(stashtrap_sync_failures_total[7d])) or vector(0)";
      decimals = 0;
      description = "Crawls that ended in an error this week. Rate limiting and an expired API key both land here.";
      thresholds = [
        {
          color = "green";
          value = null;
        }
        {
          color = "red";
          value = 1;
        }
      ];
    })

    (row {
      id = 102;
      title = "Release shape";
      y = 46;
    })
    (ts {
      id = 20;
      title = "Announced size";
      x = 0;
      y = 47;
      w = 12;
      unit = "bytes";
      description = "Median and p95 of the announced release size, which is what a grab would cost.";
      targets = [
        (target "histogram_quantile(0.5, sum by (le) (rate(stashtrap_announce_size_bytes_bucket[6h])))"
          "p50"
          "A"
        )
        (target "histogram_quantile(0.95, sum by (le) (rate(stashtrap_announce_size_bytes_bucket[6h])))"
          "p95"
          "B"
        )
      ];
    })
    (ts {
      id = 21;
      title = "Files per torrent";
      x = 12;
      y = 47;
      w = 12;
      unit = "short";
      description = "p50 is a single scene; a rising p95 means packs, which one announce cannot describe — the file table is what phase two matches per file.";
      targets = [
        (target "histogram_quantile(0.5, sum by (le) (rate(stashtrap_announce_files_bucket[6h])))" "p50"
          "A"
        )
        (target "histogram_quantile(0.95, sum by (le) (rate(stashtrap_announce_files_bucket[6h])))" "p95"
          "B"
        )
      ];
    })

    (row {
      id = 103;
      title = "Logs";
      y = 56;
    })
    (logs {
      id = 31;
      title = "Grabs";
      x = 0;
      y = 57;
      h = 8;
      expr = ''{host="stashtrap"} | json | _SYSTEMD_UNIT = "stashtrap.service" | json decision="decision", name="announce_name", names="names", performers="favourite_performers", studios="favourite_studios", score="score" | decision = "accept" | line_format "{{.name}}\n    matched {{.names}} -> performers={{.performers}} studios={{.studios}} score={{.score}}"'';
      description = "Every accepted release and why: the text that matched, the followed entities behind it, and the evidence score. This is the panel to read when the accept rate moves — a grab that cannot be explained from its own line is the bug.";
    })
    (logs {
      id = 32;
      title = "Problems";
      x = 0;
      y = 65;
      h = 8;
      expr = ''{host="stashtrap"} | json | _SYSTEMD_UNIT = "stashtrap.service" | json level="level", msg="message" | level =~ "WARN|ERROR" | line_format "{{.level}} {{.msg}}"'';
      description = "Warnings and errors only: torrent parse failures, catalogue query errors, failed syncs. Empty is the expected state.";
    })
    (logs {
      id = 30;
      title = "stashtrap";
      x = 0;
      y = 73;
      expr = ''{host="stashtrap"} | json | _SYSTEMD_UNIT = "stashtrap.service" | line_format "{{.message}}"'';
      description = "Everything the service records: one line per announce with its decision and reading, plus sync progress.";
    })
  ];
}
