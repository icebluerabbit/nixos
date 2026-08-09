# New pull request on a watched repo → #pull-requests. State is the newest PR's
# title; only the newest is exposed, so PRs opened between polls are missed.
{ lib }:
let
  gh = import ../_github-repos.nix { inherit lib; };
  entities = gh.entityMap "latest_pull_request";
  guard = [
    "unavailable"
    "unknown"
  ];
in
{
  id = "github_pull_request";
  alias = "GitHub: Pull Request";
  triggers = [
    {
      trigger = "state";
      entity_id = builtins.attrNames entities;
      not_from = guard;
      not_to = guard;
    }
  ];
  conditions = [ ];
  variables.repo = "{{ ${builtins.toJSON entities}[trigger.entity_id] | default(trigger.entity_id) }}";
  actions = [
    {
      action = "matrix.send_message";
      data = {
        message = builtins.readFile ./msg/github/PullRequest.tmpl;
        data.format = "html";
        target = [ "#pull-requests:matrix.services.internal" ];
      };
    }
  ];
  mode = "queued";
}
