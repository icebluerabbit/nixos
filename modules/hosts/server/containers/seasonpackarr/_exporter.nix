/*
  Vector metrics for seasonpackarr.

  seasonpackarr has no /metrics and no stats API — its routes are /api/pack,
  /api/parse, /api/token and /api/healthz/*. What it does have is a notifier, and
  although upstream calls it "discord" it performs no URL validation
  whatsoever: internal/notification/discord.go builds the embed and hands the
  configured string straight to http.NewRequest, accepting 200 or 204 back.
  So it is pointed at this sidecar's `webhook_in` (declared in
  seasonpackarr.nix, which also republishes it to MQTT for Home Assistant).

  That is worth far more than parsing the log, because the embed's title is
  BuildTitle(statusCode) — the upstream StatusCode enum verbatim:

    successful match · no matching releases in client · number of matches below
    threshold · resolution did not match · source did not match · release group
    did not match · season did not match · episode did not match · HDR metadata
    did not match · release already in client · release is not a season pack ·
    could not create hardlinks · could not match episodes to files in pack · …

  i.e. every pack outcome as a bounded label, with no prose matching involved.

  The journal is still read, but only for the two things the notifier does not
  carry: the per-episode comparison detail behind a rejection, and the match
  ratio. Those lines are JSON (seasonpackarr logs structured), so they are
  parsed, not regexed:

    {"level":"info","module":"processor","release":"…","time":"…",
     "message":"resolution did not match: request(… => 1080p), client(… => 2160p)"}
    {"level":"info","module":"processor","message":"found 3/8 (37.50%) episodes in client"}

  Which outcomes reach the wire at all is `notificationLevel`. INFO is included
  in seasonpackarr.nix so the rejection outcomes are counted here; the MQTT sink
  narrows that back down for Home Assistant.
*/
{ port }:
let
  base = "http://127.0.0.1:${toString port}";
in
{
  prometheusInputs = [
    "seasonpackarr_up_metric"
    "seasonpackarr_pack_metric"
    "seasonpackarr_line_metric"
    "seasonpackarr_mismatch_metric"
    "seasonpackarr_ratio_metric"
    "seasonpackarr_unauthorized_metric"
  ];

  extraSettings = {
    # /api/healthz/{liveness,readiness} are the only two paths middleware.go
    # exempts from the API token, so the sidecar needs no credential.
    sources.seasonpackarr_health = {
      type = "http_client";
      endpoint = "${base}/api/healthz/readiness";
      scrape_interval_secs = 60;
      scrape_timeout_secs = 15;
    };

    transforms = {
      # ── Liveness ────────────────────────────────────────────────────────
      # An unreachable endpoint emits nothing rather than a 0, so this goes
      # stale instead of dropping; vector_build_info is the real heartbeat.
      seasonpackarr_up_parse = {
        type = "remap";
        inputs = [ "seasonpackarr_health" ];
        drop_on_abort = true;
        source = ''
          .up = 1
        '';
      };
      seasonpackarr_up_metric = {
        type = "log_to_metric";
        inputs = [ "seasonpackarr_up_parse" ];
        metrics = [
          {
            type = "gauge";
            field = "up";
            name = "seasonpackarr_up";
          }
        ];
      };

      # ── Pack outcomes, from the notifier ────────────────────────────────
      seasonpackarr_pack_parse = {
        type = "remap";
        inputs = [ "webhook_in" ];
        drop_on_abort = true;
        source = ''
          n = parse_json!(string!(.message))
          embeds = array(n.embeds) ?? []
          if length(embeds) == 0 { abort }
          embed = object!(embeds[0])
          title = string(embed.title) ?? ""
          if title == "" { abort }

          client = "unknown"
          for_each(array(embed.fields) ?? []) -> |_i, f| {
            field = object!(f)
            if (string(field.name) ?? "") == "Client" {
              client = string(field.value) ?? "unknown"
            }
          }

          . = {
            "outcome": replace(downcase(title), " ", "_"),
            "client": client,
            "count": 1,
          }
        '';
      };
      seasonpackarr_pack_metric = {
        type = "log_to_metric";
        inputs = [ "seasonpackarr_pack_parse" ];
        metrics = [
          {
            type = "counter";
            field = "count";
            name = "seasonpackarr_packs_total";
            tags = {
              outcome = "{{outcome}}";
              client = "{{client}}";
            };
          }
        ];
      };

      # ── Journal: unit filter, then JSON ─────────────────────────────────
      # `journal` carries the whole container — pam, systemd and Vector's own
      # logs — and Vector's lines are not JSON, so both filters are load-bearing.
      seasonpackarr_parse = {
        type = "remap";
        inputs = [ "journal" ];
        drop_on_abort = true;
        source = ''
          if (string(._SYSTEMD_UNIT) ?? "") != "seasonpackarr.service" { abort }
          j = parse_json(string!(.message)) ?? null
          if j == null { abort }
          . = {
            "level": string(j.level) ?? "unknown",
            "module": string(j.module) ?? "none",
            "msg": string(j.message) ?? "",
            "count": 1,
          }
        '';
      };
      seasonpackarr_line_metric = {
        type = "log_to_metric";
        inputs = [ "seasonpackarr_parse" ];
        metrics = [
          {
            type = "counter";
            field = "count";
            name = "seasonpackarr_log_lines_total";
            tags = {
              level = "{{level}}";
              module = "{{module}}";
            };
          }
        ];
      };

      # ── Which attribute rejected an episode ─────────────────────────────
      # "<attribute> did not match: request(…), client(…)" — one line per
      # episode compared, so this shows why a pack missed, not just that it did.
      seasonpackarr_mismatch_parse = {
        type = "remap";
        inputs = [ "seasonpackarr_parse" ];
        drop_on_abort = true;
        source = ''
          m = parse_regex(string!(.msg), r'^(?P<attr>[a-zA-Z ]+) did not match:') ?? null
          if m == null { abort }
          .attribute = replace(downcase(string!(m.attr)), " ", "_")
        '';
      };
      seasonpackarr_mismatch_metric = {
        type = "log_to_metric";
        inputs = [ "seasonpackarr_mismatch_parse" ];
        metrics = [
          {
            type = "counter";
            field = "count";
            name = "seasonpackarr_mismatches_total";
            tags.attribute = "{{attribute}}";
          }
        ];
      };

      # ── How much of a season was already held ───────────────────────────
      seasonpackarr_ratio_parse = {
        type = "remap";
        inputs = [ "seasonpackarr_parse" ];
        drop_on_abort = true;
        source = ''
          m = parse_regex(
            string!(.msg),
            r'^found (?P<have>\d+)/(?P<want>\d+) \((?P<pct>[\d.]+)%\) episodes in client'
          ) ?? null
          if m == null { abort }
          .have = to_float(m.have) ?? 0.0
          .want = to_float(m.want) ?? 0.0
          .pct = to_float(m.pct) ?? 0.0
        '';
      };
      seasonpackarr_ratio_metric = {
        type = "log_to_metric";
        inputs = [ "seasonpackarr_ratio_parse" ];
        metrics = [
          {
            type = "gauge";
            field = "pct";
            name = "seasonpackarr_match_ratio_percent";
          }
          {
            type = "counter";
            field = "have";
            name = "seasonpackarr_episodes_matched_total";
            increment_by_value = true;
          }
          {
            type = "counter";
            field = "want";
            name = "seasonpackarr_episodes_wanted_total";
            increment_by_value = true;
          }
        ];
      };

      # ── Announces arriving without the API token ────────────────────────
      seasonpackarr_unauthorized_parse = {
        type = "remap";
        inputs = [ "seasonpackarr_parse" ];
        drop_on_abort = true;
        source = ''
          m = parse_regex(
            string!(.msg),
            r'^unauthorized access attempt without API token from IP: (?P<ip>\S+)'
          ) ?? null
          if m == null { abort }
          .ip = m.ip
        '';
      };
      seasonpackarr_unauthorized_metric = {
        type = "log_to_metric";
        inputs = [ "seasonpackarr_unauthorized_parse" ];
        metrics = [
          {
            type = "counter";
            field = "count";
            name = "seasonpackarr_unauthorized_total";
            tags.ip = "{{ip}}";
          }
        ];
      };
    };
  };
}
