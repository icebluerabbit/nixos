# autobrr WEBHOOK-notification → #autobrr. Single trigger (the webhook); branch on
# the push event. On a reject, inject a hidden <!-- Ref: N --> into the card, then
# loop waiting for a 👍 (matrix_command autobrr_grab — see matrix-commands/): each
# reaction wakes every parked reject, so fetch the reacted card by event_id and
# re-wait unless its ref marker is ours. Bounded by an absolute 30m deadline so a
# busy room can't keep an instance alive past that. Parallel mode (max 100) so
# pending rejects wait independently without exhausting run slots.
_: {
  id = "autobrr_webhook";
  alias = "autobrr Webhook";
  triggers = [
    {
      trigger = "mqtt";
      topic = "homelab/notify/autobrr";
      qos = 1;
    }
  ];
  conditions = [ ];
  actions = [
    {
      choose = [
        {
          # cross-seed announce plumbing (WEBHOOK action) — this is just the POST-ack,
          # not a grab. Swallow it (no message) so it doesn't spam a misleading
          # "accepted"; cross-seed's own inject notifications go to #cross-seed.
          conditions = [
            {
              condition = "template";
              value_template = "{{ trigger.payload_json.event == 'action.approved' and trigger.payload_json.data.action.type == 'WEBHOOK' }}";
            }
          ];
          sequence = [ ];
        }
        {
          conditions = [
            {
              condition = "template";
              value_template = "{{ trigger.payload_json.event == 'action.approved' }}";
            }
          ];
          sequence = [
            {
              action = "matrix.send_message";
              data = {
                message = builtins.readFile ./msg/autobrr/Accepted.tmpl;
                data.format = "html";
                target = [ "#autobrr:matrix.services.internal" ];
              };
            }
          ];
        }
        {
          # cross-seed announce (WEBHOOK) times out waiting for cross-seed's sync
          # search ack, but the announce IS delivered + processed — cosmetic only.
          # Swallow it, same as we swallow the WEBHOOK approved ack above.
          conditions = [
            {
              condition = "template";
              value_template = "{{ trigger.payload_json.event == 'action.error' and trigger.payload_json.data.action.type == 'WEBHOOK' }}";
            }
          ];
          sequence = [ ];
        }
        {
          conditions = [
            {
              condition = "template";
              value_template = "{{ trigger.payload_json.event == 'action.error' }}";
            }
          ];
          sequence = [
            {
              action = "matrix.send_message";
              data = {
                message = builtins.readFile ./msg/autobrr/Error.tmpl;
                data.format = "html";
                target = [ "#autobrr:matrix.services.internal" ];
              };
            }
          ];
        }
        {
          conditions = [
            {
              condition = "template";
              value_template = "{{ trigger.payload_json.event == 'action.rejected' }}";
            }
          ];
          sequence = [
            {
              variables = {
                grab_id = "{{ range(10000, 99999) | random }}";
                deadline = "{{ (now().timestamp() + 1800) | int }}";
              };
            }
            {
              action = "matrix.send_message";
              data = {
                message = builtins.readFile ./msg/autobrr/Rejected.tmpl;
                data.format = "html";
                target = [ "#autobrr:matrix.services.internal" ];
              };
            }
            {
              repeat = {
                sequence = [
                  {
                    wait_for_trigger = [
                      {
                        trigger = "event";
                        event_type = "matrix_command";
                        event_data.command = "autobrr_grab";
                      }
                    ];
                    # Time left until the deadline, never more — so non-matching
                    # reactions can't reset us past the 1h cap.
                    timeout = "{{ [(deadline | int) - now().timestamp(), 1] | max }}";
                    continue_on_timeout = true;
                  }
                  {
                    "if" = [
                      {
                        condition = "template";
                        value_template = "{{ wait.trigger is not none }}";
                      }
                    ];
                    "then" = [
                      {
                        action = "rest_command.matrix_login";
                        data.password = "!secret matrix_ha_bot_password";
                        response_variable = "login";
                      }
                      {
                        action = "rest_command.matrix_get_event";
                        data = {
                          room = "{{ wait.trigger.event.data.room }}";
                          event_id = "{{ wait.trigger.event.data.event_id }}";
                          token = "{{ login.content.access_token }}";
                        };
                        response_variable = "ev";
                      }
                      {
                        variables.matched = "{{ ('<!-- Ref: ' ~ grab_id ~ ' -->') in (ev.content.content.body | default('')) }}";
                      }
                    ];
                  }
                ];
                until = [
                  {
                    condition = "template";
                    value_template = "{{ (matched | default(false)) or (now().timestamp() >= (deadline | int)) }}";
                  }
                ];
              };
            }
            {
              "if" = [
                {
                  condition = "template";
                  value_template = "{{ matched | default(false) }}";
                }
              ];
              "then" = [
                {
                  action = "rest_command.qbt_add";
                  data = {
                    url = "{{ trigger.payload_json.data.release.download_url | default(trigger.payload_json.data.release.magnet_uri) }}";
                    category = ''
                      {%- set r = trigger.payload_json.data.release -%}
                      {%- set t = (r.type | default("")) | lower -%}
                      {%- if t == 'movie' -%}movies
                      {%- elif t in ['series','episode'] or r.season or r.episode -%}tv
                      {%- elif t == 'music' or r.artists -%}music
                      {%- elif t in ['book','audiobook','comic','magazine','education'] -%}books
                      {%- else -%}misc
                      {%- endif -%}'';
                  };
                }
                {
                  action = "matrix.send_message";
                  data = {
                    message = builtins.readFile ./msg/autobrr/Approved.tmpl;
                    data.format = "html";
                    target = [ "#autobrr:matrix.services.internal" ];
                  };
                }
              ];
            }
          ];
        }
      ];
      default = [
        {
          action = "matrix.send_message";
          data = {
            message = "{{ trigger.payload_json | tojson(indent=2) }}";
            target = [ "#autobrr:matrix.services.internal" ];
          };
        }
      ];
    }
  ];
  mode = "parallel";
  # Parked reject-approval waits each hold a run slot until their deadline; the
  # default cap of 10 gets exhausted under reject volume and silently drops new
  # notifications (failed_max_runs). 100 slots + 30m deadline keeps headroom.
  max = 100;
}
