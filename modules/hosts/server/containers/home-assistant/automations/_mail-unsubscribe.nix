/*
  👍 on an unsubscribable #inbox card → RFC 8058 one-click unsubscribe.

  Unlike the autobrr grab this needs no parked instance and no deadline: the card
  itself carries entry_id|uid|url in a hidden marker, so a standing automation can
  rebuild everything from the reacted event and a 👍 stays valid indefinitely.

  Order matters. The POST goes first and the IMAP move is best-effort after it —
  uid is folder-scoped, so a message already archived in Gmail can no longer be
  moved, and that must not stop the unsubscribe or turn it into a failure card.
*/
_: {
  id = "mail_unsubscribe";
  alias = "Mail: Unsubscribe";
  triggers = [
    {
      trigger = "event";
      event_type = "matrix_command";
      event_data.command = "mail_unsub";
    }
  ];
  conditions = [ ];
  actions = [
    {
      action = "rest_command.matrix_login";
      data.password = "!secret matrix_ha_bot_password";
      response_variable = "login";
    }
    {
      action = "rest_command.matrix_get_event";
      data = {
        room = "{{ trigger.event.data.room }}";
        event_id = "{{ trigger.event.data.event_id }}";
        token = "{{ login.content.access_token }}";
      };
      response_variable = "ev";
    }
    {
      variables.marker = ''
        {{ (ev.content.content.body | default("")) | regex_findall('<!-- Unsub: ([^|]+)\|([^|]+)\|(https://[^ ]+) -->') }}
      '';
    }
    # A 👍 on any other card in the room lands here too — no marker, nothing to do.
    {
      condition = "template";
      value_template = "{{ (marker | count) > 0 }}";
    }
    {
      variables = {
        entry = "{{ marker[0][0] }}";
        uid = "{{ marker[0][1] }}";
        unsub_url = "{{ marker[0][2] }}";
      };
    }
    {
      action = "rest_command.mail_unsubscribe";
      data.url = "{{ unsub_url }}";
      response_variable = "resp";
    }
    {
      "if" = [
        {
          condition = "template";
          value_template = "{{ 200 <= (resp.status | default(0) | int) < 300 }}";
        }
      ];
      "then" = [
        {
          action = "imap.move";
          data = {
            entry = "{{ entry }}";
            uid = "{{ uid }}";
            target_folder = "[Gmail]/Trash";
            seen = true;
          };
          continue_on_error = true;
        }
        {
          action = "matrix.send_message";
          data = {
            message = builtins.readFile ./msg/mail/Unsubscribed.tmpl;
            data.format = "html";
            target = [ "#inbox:matrix.services.internal" ];
          };
        }
      ];
      "else" = [
        {
          action = "matrix.send_message";
          data = {
            message = builtins.readFile ./msg/mail/UnsubscribeFailed.tmpl;
            data.format = "html";
            target = [ "#inbox:matrix.services.internal" ];
          };
        }
      ];
    }
  ];
  mode = "queued";
  max = 10;
}
