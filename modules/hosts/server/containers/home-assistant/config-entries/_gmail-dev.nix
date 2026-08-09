# Gmail (dev) IMAP entry — Workspace account on the dev domain. Same shape as
# _gmail-personal.nix; see there for why every tunable lives under `data`.
_: {
  entry_id = "a1fd425e-3223-4a67-9c1c-fe8b1217cb67";
  domain = "imap";
  title = "Gmail (dev)";
  data = {
    server = "imap.gmail.com";
    port = 993;
    username = "$GMAIL_DEV_USER";
    password = "$GMAIL_DEV_APP_PASSWORD";
    charset = "utf-8";
    verify_ssl = true;
    ssl_cipher_list = "python_default";
    folder = "INBOX";
    search = "UnSeen UnDeleted";
    event_message_data = [ ];
    max_message_size = 2048;
    enable_push = true;
  };
}
