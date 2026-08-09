# A burst of 401s out of Authelia's authz endpoint: someone is being refused, over and
# over, at the front door of everything exposed. A handful is normal (an expired
# session, a bookmarked URL); a sustained rate is not.
{ promRule, ... }:
{
  group = "infra";
  rules = [
    (promRule {
      uid = "authelia_auth_failures";
      title = "Authelia auth failures";
      expr = ''sum(rate(authelia_authz{code="401"}[5m])) * 60'';
      op = "gt";
      value = 10;
      for = "10m";
      severity = "warning";
      category = "auth";
      noDataState = "OK";
      keepFiringFor = "10m";
      summary = "Authelia is refusing >10 requests/min";
      description = "Authelia has returned more than 10 401s per minute for 10m. Either a service is looping against a dead session, or something is trying credentials.";
    })
  ];
}
