# Gmail (personal) IMAP entry — broad INBOX watch feeding #inbox via
# automations/_mail-relay.nix. Credentials expand from seedEnvironmentFile at
# seed time; the Nix store only sees the literal $GMAIL_* references.
#
# The imap options flow writes back into `data` (not `options`), so every
# tunable lives there. Entry uniqueness is (username, server, folder, search) —
# narrower entries added later must differ in at least one of those.
_: {
  entry_id = "6a9b30e3-4cb2-490a-8de6-40a8f10ba4ec";
  domain = "imap";
  title = "Gmail (personal)";
  data = {
    server = "imap.gmail.com";
    port = 993;
    username = "$GMAIL_PERSONAL_USER";
    password = "$GMAIL_PERSONAL_APP_PASSWORD";
    charset = "utf-8";
    verify_ssl = true;
    ssl_cipher_list = "python_default";
    # Gmail files spam under [Gmail]/Spam, so INBOX already excludes it.
    folder = "INBOX";
    search = "UnSeen UnDeleted";
    # Body stays out of the event bus — HTML mail blows past the coordinator's
    # 32168-byte event cap on its own, and an oversized event is dropped whole
    # (the message is then never relayed at all). Headers are in, because they
    # are only in scope for custom_event_data_template when listed here, and
    # List-Unsubscribe lives there. Header blocks run a few KB, so the cap is
    # still the thing to watch: a skip logs "Custom imap_content event skipped".
    event_message_data = [ "headers" ];
    max_message_size = 2048;
    # unsub URL + body preview, both computed at the source. See the template.
    custom_event_data_template = import ./_mail-custom-template.nix;
    # IMAP IDLE; false falls back to a 10s poll.
    enable_push = true;
  };
}
