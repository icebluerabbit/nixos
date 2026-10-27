/*
  Daily DNS summary → #dns Matrix room.

  Pi-hole's own counters are the point of this: query volume and block ratio are
  interesting in aggregate and worthless per-event (the resolver handles thousands of
  queries a day, so anything finer than a digest is noise). Numbers are read from
  Prometheus rather than Pi-hole's API so the message survives FTL being restarted —
  remote_write has already persisted the series.

  Fires at 09:00, before Pi-hole's own counters roll over at midnight would make
  "today" mean the wrong day: the *_today metrics are read while they still describe
  a full day of traffic.
*/
_:
let
  # One call, five series. PromQL `or` cannot be used to union these — every series
  # carries an identical label set, so the right-hand sides collide with the left and
  # get dropped, silently returning a single metric.
  statsQuery = "{__name__=~\"pihole_(dns_queries_today|ads_blocked_today|ads_percentage_today|domains_being_blocked|unique_clients)\"}";
in
{
  id = "dns_digest";
  alias = "DNS Daily Digest";
  triggers = [
    {
      trigger = "time";
      at = "09:00:00";
    }
  ];
  conditions = [ ];
  actions = [
    {
      action = "rest_command.prometheus_query";
      data.query = statsQuery;
      response_variable = "stats";
    }
    {
      action = "rest_command.prometheus_query";
      data.query = "topk(3, pihole_top_ads)";
      response_variable = "top_ads";
    }
    {
      action = "rest_command.prometheus_query";
      data.query = "topk(3, pihole_top_sources)";
      response_variable = "top_sources";
    }
    # A resolver that answered nothing all day means the exporter or FTL is gone,
    # which the pihole_down alert already says in this same room. Skip rather than
    # post a row of zeroes next to it.
    {
      condition = "template";
      value_template = "{{ (stats.content.data.result | default([]) | count) > 0 }}";
    }
    {
      action = "matrix.send_message";
      data = {
        message = builtins.readFile ./msg/dns/Digest.tmpl;
        data.format = "html";
        target = [ "#dns:matrix.services.internal" ];
      };
    }
  ];
  mode = "single";
}
