# autobrr's IRC announce connection. This is the alert autobrr most needs: the service
# stays up, the web UI stays green, and it simply stops seeing announces — so nothing
# gets grabbed and the only symptom is an absence of activity nobody notices for days.
#
# Multiplied rather than OR'd: connected-but-unhealthy is just as dead as disconnected.
{ promRule, ... }:
{
  group = "media";
  rules = [
    (promRule {
      uid = "autobrr_irc_down";
      title = "autobrr IRC down";
      expr = "autobrr_irc_connected * autobrr_irc_healthy";
      op = "lt";
      value = 1;
      for = "10m";
      severity = "warning";
      category = "autobrr";
      noDataState = "OK";
      summary = "autobrr IRC announce connection is down";
      description = "autobrr's IRC connection has been down or unhealthy for 10m. autobrr is still running but sees no announces — nothing will be grabbed.";
    })
  ];
}
