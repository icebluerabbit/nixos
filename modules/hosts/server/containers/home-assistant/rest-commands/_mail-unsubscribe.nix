# RFC 8058 one-click unsubscribe: POST the List-Unsubscribe URL with the fixed
# form body and nothing else. The URL is caller-supplied (read back off the card
# marker) but was already gated to https + One-Click by the imap custom template,
# so this never fetches a bare GET link or a mailto:. Timeout is generous — ESP
# unsubscribe endpoints are frequently slow — and the caller reports the status.
_: {
  mail_unsubscribe = {
    url = "{{ url }}";
    method = "POST";
    content_type = "application/x-www-form-urlencoded";
    payload = "List-Unsubscribe=One-Click";
    timeout = 20;
  };
}
