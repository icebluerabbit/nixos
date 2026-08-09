# New commit on a watched repo's default branch → #commits. State is the commit
# message (truncated to 255 chars by the integration). Noisiest of the GitHub
# relays by a wide margin — one message per push per repo.
{ lib }:
let
  gh = import ../_github-repos.nix { inherit lib; };
  entities = gh.entityMap "latest_commit";
  guard = [
    "unavailable"
    "unknown"
  ];
in
{
  id = "github_commit";
  alias = "GitHub: Commit";
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
        message = builtins.readFile ./msg/github/Commit.tmpl;
        data.format = "html";
        target = [ "#commits:matrix.services.internal" ];
      };
    }
  ];
  mode = "queued";
}
