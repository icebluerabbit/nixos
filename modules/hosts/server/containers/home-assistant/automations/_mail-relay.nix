# New mail in either Gmail INBOX → #inbox. The imap integration fires one
# imap_content event per newly-seen message; `initial` is false when the same
# Message-ID is re-processed (reconnect / flag churn), so it gates duplicates.
# Matched on entry_id rather than username so later narrower entries (OTP,
# receipts) posting to their own rooms don't also land here. The account label
# is resolved into a variable so the message template stays static.
_:
let
  accounts = {
    "6a9b30e3-4cb2-490a-8de6-40a8f10ba4ec" = "personal";
    "a1fd425e-3223-4a67-9c1c-fe8b1217cb67" = "dev";
  };
in
{
  id = "mail_inbox_relay";
  alias = "Mail: Inbox Relay";
  triggers = [
    {
      trigger = "event";
      event_type = "imap_content";
    }
  ];
  conditions = [
    {
      condition = "template";
      value_template = "{{ trigger.event.data.entry_id in ${builtins.toJSON accounts} }}";
    }
    {
      condition = "template";
      value_template = "{{ trigger.event.data.initial }}";
    }
  ];
  variables.account = "{{ ${builtins.toJSON accounts}[trigger.event.data.entry_id] | default('unknown') }}";
  actions = [
    {
      action = "matrix.send_message";
      data = {
        message = builtins.readFile ./msg/mail/Inbox.tmpl;
        data.format = "html";
        target = [ "#inbox:matrix.services.internal" ];
      };
    }
  ];
  mode = "queued";
}
