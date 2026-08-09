# New release on a watched repo → #releases. The github integration polls; the
# latest_release sensor's state is the release name, so a change means a new
# release. not_from/not_to keep restarts and repos with no releases quiet —
# those sensors report `unavailable` (avabl_fn) until one exists.
{ lib }:
let
  gh = import ../_github-repos.nix { inherit lib; };
  entities = gh.entityMap "latest_release";
  guard = [
    "unavailable"
    "unknown"
  ];
in
{
  id = "github_release";
  alias = "GitHub: Release";
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
        message = builtins.readFile ./msg/github/Release.tmpl;
        data.format = "html";
        target = [ "#releases:matrix.services.internal" ];
      };
    }
  ];
  mode = "queued";
}
