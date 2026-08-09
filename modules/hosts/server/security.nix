# security — host configuration override for sops secrets and ssh daemon bindings
_: {
  den.aspects.hosts._.server._.security = {
    aspect-name = "hosts.server.security";
    includes = [ ];

    nixos =
      { config, ... }:
      {
        sops = {
          secrets = {
            pgadmin-password = { };
            wifi-passphrase = { };
            sonarr-apikey = { };
            radarr-apikey = { };
            bazarr-apikey = { };
            opensubtitles-username = { };
            opensubtitles-password = { };
            subdl-api-key = { };
            # addic7ed disabled (needs anti-captcha/cookies) — re-enable when wired.
            # addic7ed-username = { };
            # addic7ed-password = { };
            lidarr-apikey = { };
            readarr-apikey = { };
            lazylibrarian-apikey = { };
            prowlarr-apikey = { };
            mam-username = {
              mode = "0444";
            };
            mam-irc-password = {
              mode = "0444";
            };
            # autobrr's MAM indexer session cookie (real-IP). mousetrap uses its own
            # (mam-api-id) since MAM rotates mam_id per request — one owner per session.
            mam-autobrr-id = { };
            # mousetrap's MAM API session cookie (jsonLoad poller; rotating, sole owner).
            # 0444: read directly by the mousetrap container's non-root service user.
            mam-api-id.mode = "0444";
            # Public-network IRC nicks (Libera.Chat, Rizon). Not sensitive like a
            # tracker username, but kept out of the Nix store for the same reason.
            libera-nickserv-password = {
              mode = "0444";
            };
            rizon-nickserv-password = {
              mode = "0444";
            };
            libera-username = {
              mode = "0444";
            };
            rizon-username = {
              mode = "0444";
            };
            snoonet-username = {
              mode = "0444";
            };
            pretome-username = {
              mode = "0444";
            };
            pretome-irc-password = {
              mode = "0444";
            };
            pretome-password = { };
            pretome-pin = { };
            # 32-hex RSS feed key — builds autobrr's download URL. Distinct from
            # pretome-irc-key (16-hex), which is only the GateKeeper channel grant.
            pretome-rss-key = { };
            pretome-irc-key = { };
            unifi-controller-username = { };
            unifi-controller-password = { };
            unifi-drive-username = { };
            unifi-drive-password = { };
            youtarr-mysql-password = { };
            grafana-username = { };
            grafana-password = { };
            home-assistant-llat = { };
            # Gmail IMAP app passwords (2FA app passwords, not account passwords)
            # for the home-assistant imap entries feeding #inbox.
            gmail-personal-user = { };
            gmail-personal-app-password = { };
            gmail-dev-user = { };
            gmail-dev-app-password = { };
            # GitHub PAT for the home-assistant github integration. Classic token
            # with NO scopes selected — every watched repo is public, and reading
            # public repo data plus /user needs no scope. public_repo would grant
            # write access to every public repo, which this never needs.
            #
            # NOT a copy of the user-secret of the same name (that one is the
            # full-access token behind gh / git / the github MCP server / gitea
            # push-mirroring). Same key, different file, deliberately weaker.
            github-access-token-homeassistant = { };
            home-latitude = { };
            home-longitude = { };
            zwave-key-s0 = { };
            zwave-key-s2-unauthenticated = { };
            zwave-key-s2-authenticated = { };
            zwave-key-s2-access-control = { };
            zwave-key-s2-authenticated-lr = { };
            zwave-key-s2-access-control-lr = { };
            anthropic-api-key = { };
            openrouter-api-key = { };
            lldap-jwt-secret = {
              mode = "0444";
            };
            lldap-admin-password = {
              mode = "0444";
            };
            authelia-ldap-password = {
              mode = "0444";
            };
            authelia-jwt-secret = {
              mode = "0444";
            };
            authelia-storage-encryption-key = {
              mode = "0444";
            };
            authelia-web-admin-password-hash = { };
            authelia-oidc-hmac-secret = {
              mode = "0444";
            };
            authelia-oidc-jwk-rsa-key = {
              mode = "0444";
            };
            caddy-tls-key = {
              mode = "0444";
            };
            trakt-auth-user = { };
            matrix-admin-password = { };
            matrix-ha-bot-password = { };
            matrix-hermes-password = { };
            hermes-api-key = { };
            jellyfin-apikey = { };
            cross-seed-apikey = { };
            torrentleech-username = {
              mode = "0444";
            };
            torrentleech-irc-password = {
              mode = "0444";
            };
            torrentleech-irc-key = {
              mode = "0444";
            };
            torrentleech-password = { };
            torrentleech-rss-key = { };
            digitalcore-apikey = { };
            digitalcore-passkey = { };
            retrotoon-passkey = { };
            digitalcore-username = {
              mode = "0444";
            };
            digitalcore-irc-key = {
              mode = "0444";
            };
            retrotoon-username = {
              mode = "0444";
            };
            retrotoon-irc-password = {
              mode = "0444";
            };
            autobrr-session-secret = { };
            autobrr-api-token = { };
            # Cleanuparr mints its API key with the admin account during first-run
            # setup and only serves it from the API, so the password is needed on
            # every boot to log in and re-read the key — not just once.
            cleanuparr-admin-password = { };
            # gitea token for the forgejo-vscode extension in code-server (browse/clone
            # in the IDE); pushes go over SSH. Host-decryptable copy of the same value
            # the user-secret holds for Terraform. 0400 → read via LoadCredential.
            gitea-api-token = { };
            slskd-slsk-username = { };
            slskd-slsk-password = { };
            # Home Assistant long-lived access token for the Homepage HA widget.
            # Host-decryptable copy of the user-secret of the same name.
            home-assistant-llat = { };
            # Grafana service account token for the hermes grafana MCP server.
            # Host-decryptable copy of the user-secret of the same name.
            grafana-token = { };
          };
          templates = {
            "tuwunel-env".content = ''
              TUWUNEL_ADMIN_PASS=${config.sops.placeholder.matrix-admin-password}
              TUWUNEL_HA_BOT_PASS=${config.sops.placeholder.matrix-ha-bot-password}
              TUWUNEL_HERMES_PASS=${config.sops.placeholder.matrix-hermes-password}
            '';
            # Secrets only — the non-secret MATRIX_* vars (homeserver, MXID, allowed
            # users) come from containerConfigs.hermes.
            "hermes-env".content = ''
              OPENROUTER_API_KEY=${config.sops.placeholder.openrouter-api-key}
              MATRIX_PASSWORD=${config.sops.placeholder.matrix-hermes-password}
              API_SERVER_KEY=${config.sops.placeholder.hermes-api-key}
              GRAFANA_SERVICE_ACCOUNT_TOKEN=${config.sops.placeholder.grafana-token}
              HOMEASSISTANT_TOKEN=${config.sops.placeholder.home-assistant-llat}
            '';
            "jellyfin-env".content = ''
              JELLYFIN_API_KEY=${config.sops.placeholder.jellyfin-apikey}
            '';
            "youtarr-env".content = ''
              DB_PASSWORD=${config.sops.placeholder.youtarr-mysql-password}
              JELLYFIN_API_KEY=${config.sops.placeholder.jellyfin-apikey}
            '';
            "grafana-env".content = ''
              GF_SECURITY_ADMIN_USER=${config.sops.placeholder.grafana-username}
              GF_SECURITY_ADMIN_PASSWORD=${config.sops.placeholder.grafana-password}
            '';
            "homepage-env".content = ''
              HOMEPAGE_VAR_SONARR_KEY=${config.sops.placeholder.sonarr-apikey}
              HOMEPAGE_VAR_BAZARR_KEY=${config.sops.placeholder.bazarr-apikey}
              HOMEPAGE_VAR_RADARR_KEY=${config.sops.placeholder.radarr-apikey}
              HOMEPAGE_VAR_JELLYFIN_KEY=${config.sops.placeholder.jellyfin-apikey}
              HOMEPAGE_VAR_LIDARR_KEY=${config.sops.placeholder.lidarr-apikey}
              HOMEPAGE_VAR_READARR_KEY=${config.sops.placeholder.readarr-apikey}
              HOMEPAGE_VAR_PROWLARR_KEY=${config.sops.placeholder.prowlarr-apikey}
              HOMEPAGE_VAR_CROSSSEED_KEY=${config.sops.placeholder.cross-seed-apikey}
              HOMEPAGE_VAR_AUTOBRR_KEY=${config.sops.placeholder.autobrr-api-token}
              HOMEPAGE_VAR_LATITUDE=${config.sops.placeholder.home-latitude}
              HOMEPAGE_VAR_LONGITUDE=${config.sops.placeholder.home-longitude}
              HOMEPAGE_VAR_UNIFI_CONTROLLER_USERNAME=${config.sops.placeholder.unifi-controller-username}
              HOMEPAGE_VAR_UNIFI_CONTROLLER_PASSWORD=${config.sops.placeholder.unifi-controller-password}
              HOMEPAGE_VAR_UNIFI_DRIVE_USERNAME=${config.sops.placeholder.unifi-drive-username}
              HOMEPAGE_VAR_UNIFI_DRIVE_PASSWORD=${config.sops.placeholder.unifi-drive-password}
              HOMEPAGE_VAR_GRAFANA_USERNAME=${config.sops.placeholder.grafana-username}
              HOMEPAGE_VAR_GRAFANA_PASSWORD=${config.sops.placeholder.grafana-password}
              HOMEPAGE_VAR_GITEA_TOKEN=${config.sops.placeholder.gitea-api-token}
              HOMEPAGE_VAR_HASS_TOKEN=${config.sops.placeholder.home-assistant-llat}
            '';
            "unpoller-env".content = ''
              UP_UNIFI_DEFAULT_USER=${config.sops.placeholder.unifi-controller-username}
              UP_UNIFI_DEFAULT_PASS=${config.sops.placeholder.unifi-controller-password}
            '';
            "sonarr-env".content = ''
              SONARR__AUTH__APIKEY=${config.sops.placeholder.sonarr-apikey}
              SONARR__AUTH__METHOD=External
              APIKEY=${config.sops.placeholder.sonarr-apikey}
              TRAKT_AUTH_USER=${config.sops.placeholder.trakt-auth-user}
              JELLYFIN_API_KEY=${config.sops.placeholder.jellyfin-apikey}
            '';
            "radarr-env".content = ''
              RADARR__AUTH__APIKEY=${config.sops.placeholder.radarr-apikey}
              RADARR__AUTH__METHOD=External
              APIKEY=${config.sops.placeholder.radarr-apikey}
              TRAKT_AUTH_USER=${config.sops.placeholder.trakt-auth-user}
              JELLYFIN_API_KEY=${config.sops.placeholder.jellyfin-apikey}
            '';
            "bazarr-env".content = ''
              DYNACONF_AUTH__APIKEY=${config.sops.placeholder.bazarr-apikey}
              BAZARR_API_KEY=${config.sops.placeholder.bazarr-apikey}
              SONARR_APIKEY=${config.sops.placeholder.sonarr-apikey}
              RADARR_APIKEY=${config.sops.placeholder.radarr-apikey}
              OPENSUBTITLES_USERNAME=${config.sops.placeholder.opensubtitles-username}
              OPENSUBTITLES_PASSWORD=${config.sops.placeholder.opensubtitles-password}
              SUBDL_API_KEY=${config.sops.placeholder.subdl-api-key}
            '';
            "lidarr-env".content = ''
              LIDARR__AUTH__APIKEY=${config.sops.placeholder.lidarr-apikey}
              LIDARR__AUTH__METHOD=External
              APIKEY=${config.sops.placeholder.lidarr-apikey}
              JELLYFIN_API_KEY=${config.sops.placeholder.jellyfin-apikey}
            '';
            "slskd-env".content = ''
              SLSKD_SLSK_USERNAME=${config.sops.placeholder.slskd-slsk-username}
              SLSKD_SLSK_PASSWORD=${config.sops.placeholder.slskd-slsk-password}
            '';
            "readarr-env".content = ''
              READARR__AUTH__APIKEY=${config.sops.placeholder.readarr-apikey}
              READARR__AUTH__METHOD=External
            '';
            "lazylibrarian-env".content = ''
              LAZYLIBRARIAN_API_KEY=${config.sops.placeholder.lazylibrarian-apikey}
              PROWLARR_API_KEY=${config.sops.placeholder.prowlarr-apikey}
            '';
            "autobrr-env".content = ''
              AUTOBRR_API_TOKEN=${config.sops.placeholder.autobrr-api-token}
              CROSS_SEED_APIKEY=${config.sops.placeholder.cross-seed-apikey}
              TORRENTLEECH_RSS_KEY=${config.sops.placeholder.torrentleech-rss-key}
              TORRENTLEECH_NICK=${config.sops.placeholder.torrentleech-username}
              PRETOME_RSS_KEY=${config.sops.placeholder.pretome-rss-key}
              PRETOME_NICK=${config.sops.placeholder.pretome-username}
              PRETOME_IRC_PASSWORD=${config.sops.placeholder.pretome-irc-password}
              PRETOME_IRC_KEY=${config.sops.placeholder.pretome-irc-key}
              SONARR_APIKEY=${config.sops.placeholder.sonarr-apikey}
              RADARR_APIKEY=${config.sops.placeholder.radarr-apikey}
              LIDARR_APIKEY=${config.sops.placeholder.lidarr-apikey}
              MAM_NICK=${config.sops.placeholder.mam-username}
              MAM_IRC_PASSWORD=${config.sops.placeholder.mam-irc-password}
              MAM_ID=${config.sops.placeholder.mam-autobrr-id}
              DIGITALCORE_PASSKEY=${config.sops.placeholder.digitalcore-passkey}
              DIGITALCORE_NICK=${config.sops.placeholder.digitalcore-username}
              DIGITALCORE_IRC_KEY=${config.sops.placeholder.digitalcore-irc-key}
              RETROTOON_PASSKEY=${config.sops.placeholder.retrotoon-passkey}
              RETROTOON_NICK=${config.sops.placeholder.retrotoon-username}
              RETROTOON_IRC_PASSWORD=${config.sops.placeholder.retrotoon-irc-password}
            '';
            "cleanuparr-env".content = ''
              CLEANUPARR_ADMIN_USER=admin
              CLEANUPARR_ADMIN_PASSWORD=${config.sops.placeholder.cleanuparr-admin-password}
              SONARR_APIKEY=${config.sops.placeholder.sonarr-apikey}
              RADARR_APIKEY=${config.sops.placeholder.radarr-apikey}
              LIDARR_APIKEY=${config.sops.placeholder.lidarr-apikey}
            '';
            "prowlarr-env".content = ''
              PROWLARR__AUTH__APIKEY=${config.sops.placeholder.prowlarr-apikey}
              PROWLARR__AUTH__METHOD=External
              APIKEY=${config.sops.placeholder.prowlarr-apikey}
              PRETOME_USERNAME=${config.sops.placeholder.pretome-username}
              PRETOME_PASSWORD=${config.sops.placeholder.pretome-password}
              PRETOME_PIN=${config.sops.placeholder.pretome-pin}
              TORRENTLEECH_USERNAME=${config.sops.placeholder.torrentleech-username}
              TORRENTLEECH_PASSWORD=${config.sops.placeholder.torrentleech-password}
              DIGITALCORE_APIKEY=${config.sops.placeholder.digitalcore-apikey}
              RETROTOON_PASSKEY=${config.sops.placeholder.retrotoon-passkey}
              RADARR__AUTH__APIKEY=${config.sops.placeholder.radarr-apikey}
              SONARR__AUTH__APIKEY=${config.sops.placeholder.sonarr-apikey}
              LIDARR__AUTH__APIKEY=${config.sops.placeholder.lidarr-apikey}
            '';
            "unpackerr-env".content = ''
              UN_RADARR_0_API_KEY=${config.sops.placeholder.radarr-apikey}
              UN_SONARR_0_API_KEY=${config.sops.placeholder.sonarr-apikey}
              UN_LIDARR_0_API_KEY=${config.sops.placeholder.lidarr-apikey}
            '';
            "esphome-env".content = ''
              WIFI_PASSWORD=${config.sops.placeholder.wifi-passphrase}
            '';
            "home-assistant-vector-env".content = ''
              HASS_TOKEN=${config.sops.placeholder.home-assistant-llat}
            '';
            "home-assistant-secrets" = {
              content = ''
                matrix_ha_bot_password: ${config.sops.placeholder.matrix-ha-bot-password}
                latitude: ${config.sops.placeholder.home-latitude}
                longitude: ${config.sops.placeholder.home-longitude}
              '';
              mode = "0444";
            };
            "home-assistant-seed-env".content = ''
              UNIFI_USERNAME=${config.sops.placeholder.unifi-controller-username}
              UNIFI_PASSWORD=${config.sops.placeholder.unifi-controller-password}
              GMAIL_PERSONAL_USER=${config.sops.placeholder.gmail-personal-user}
              GMAIL_PERSONAL_APP_PASSWORD=${config.sops.placeholder.gmail-personal-app-password}
              GMAIL_DEV_USER=${config.sops.placeholder.gmail-dev-user}
              GMAIL_DEV_APP_PASSWORD=${config.sops.placeholder.gmail-dev-app-password}
              GITHUB_TOKEN=${config.sops.placeholder.github-access-token-homeassistant}
            '';
            "zwave-env".content = ''
              NETWORK_KEY_S0=${config.sops.placeholder.zwave-key-s0}
              NETWORK_KEY_S2_Unauthenticated=${config.sops.placeholder.zwave-key-s2-unauthenticated}
              NETWORK_KEY_S2_Authenticated=${config.sops.placeholder.zwave-key-s2-authenticated}
              NETWORK_KEY_S2_AccessControl=${config.sops.placeholder.zwave-key-s2-access-control}
              NETWORK_KEY_S2_Authenticated_LongRange=${config.sops.placeholder.zwave-key-s2-authenticated-lr}
              NETWORK_KEY_S2_AccessControl_LongRange=${config.sops.placeholder.zwave-key-s2-access-control-lr}
            '';
            "opencode-env".content = "OPENROUTER_API_KEY=${config.sops.placeholder.openrouter-api-key}";
          };
        };
      };
  };
}
