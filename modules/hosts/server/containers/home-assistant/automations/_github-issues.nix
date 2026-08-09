# New issue on a watched repo → #issues. State is the newest issue's title.
{ lib }:
let
  gh = import ../_github-repos.nix { inherit lib; };
  entities = gh.entityMap "latest_issue";
  guard = [
    "unavailable"
    "unknown"
  ];
in
{
  id = "github_issue";
  alias = "GitHub: Issue";
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
        message = builtins.readFile ./msg/github/Issue.tmpl;
        data.format = "html";
        target = [ "#issues:matrix.services.internal" ];
      };
    }
  ];
  mode = "queued";
}
