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
    # Keep body and headers out of the event bus. The coordinator drops any
    # imap_content event over 32168 bytes outright, and HTML mail blows past
    # that with the text included.
    event_message_data = [ ];
    max_message_size = 2048;
    # IMAP IDLE; false falls back to a 10s poll.
    enable_push = true;
  };
}
