# Star count change on a watched repo → #stars. Note the entity_id suffix is
# `stars`, not the sensor key `stargazers_count` — HA slugifies the translated
# entity name ("Stars"), not the key.
{ lib }:
let
  gh = import ../_github-repos.nix { inherit lib; };
  entities = gh.entityMap "stars";
  guard = [
    "unavailable"
    "unknown"
  ];
in
{
  id = "github_stars";
  alias = "GitHub: Stars";
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
        message = builtins.readFile ./msg/github/Stars.tmpl;
        data.format = "html";
        target = [ "#stars:matrix.services.internal" ];
      };
    }
  ];
  mode = "queued";
}
