/*
  Cleanuparr's notifications → #cleanuparr.

  Cleanuparr has no plain webhook provider, so it publishes through its ntfy
  provider instead, aimed at the container's own Vector sidecar rather than a
  real ntfy server (see hosts/server/containers/cleanuparr/cleanuparr.nix). The
  sidecar republishes the body verbatim to homelab/notify/cleanuparr, so the
  payload here is an ntfy publish object: {topic, title, message, priority,
  tags}. There is no event-type field.

  Branching is therefore on a substring of the title. Cleanuparr's six notify
  kinds are NotifyStrike, NotifyQueueItemDeleted, NotifyDownloadCleaned,
  NotifyCategoryChanged, NotifySearchTriggered and NotifySearchItemGrabbed, but
  the human titles it actually sends are not declared anywhere readable in the
  shipped assembly, so these match loosely on purpose. `grabbed` is tested
  before `search` because the grab title contains both words. Anything
  unmatched falls through to the raw-JSON default rather than being dropped —
  if a card shows up as JSON, copy its title into a branch here.

  In phase 0 the only busy branch is CategoryChanged: that is the
  unlinked/dead tagging. Strike and delete cards appearing before phase 1 is
  enabled means something is striking that should not be.
*/
_:
let
  target = [ "#cleanuparr:matrix.services.internal" ];
  send = tmpl: [
    {
      action = "matrix.send_message";
      data = {
        message = builtins.readFile tmpl;
        data.format = "html";
        inherit target;
      };
    }
  ];
  # Case-insensitive substring test against the ntfy title.
  onTitle = needle: [
    {
      condition = "template";
      value_template = "{{ '${needle}' in (trigger.payload_json.title | default('') | lower) }}";
    }
  ];
in
{
  id = "cleanuparr_webhook";
  alias = "Cleanuparr Webhook";
  triggers = [
    {
      trigger = "mqtt";
      topic = "homelab/notify/cleanuparr";
      qos = 1;
    }
  ];
  conditions = [ ];
  actions = [
    {
      choose = [
        {
          conditions = onTitle "strike";
          sequence = send ./msg/cleanuparr/Strike.tmpl;
        }
        {
          conditions = onTitle "grabbed";
          sequence = send ./msg/cleanuparr/SearchItemGrabbed.tmpl;
        }
        {
          conditions = onTitle "search";
          sequence = send ./msg/cleanuparr/SearchTriggered.tmpl;
        }
        {
          conditions = onTitle "categor";
          sequence = send ./msg/cleanuparr/CategoryChanged.tmpl;
        }
        {
          conditions = onTitle "clean";
          sequence = send ./msg/cleanuparr/DownloadCleaned.tmpl;
        }
        {
          conditions = onTitle "delet";
          sequence = send ./msg/cleanuparr/QueueItemDeleted.tmpl;
        }
      ];
      default = [
        {
          action = "matrix.send_message";
          data = {
            message = "{{ trigger.payload_json | tojson(indent=2) }}";
            inherit target;
          };
        }
      ];
    }
  ];
  mode = "queued";
}
