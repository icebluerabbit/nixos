# audiobookshelf — host configuration override for audiobookshelf container
{ den, inputs, ... }:
{
  den.aspects.hosts._.server._.containers._.audiobookshelf = {
    aspect-name = "hosts.server.containers.audiobookshelf";
    includes = [
      den.aspects.containers._.audiobookshelf
      den.aspects.infra._.clients._.container-audiobookshelf
      den.aspects.infra._.clients._.container-dns
      den.aspects.infra._.networks._.media
      den.aspects.hosts._.server._.security
      den.aspects.hosts._.server._.networking._.vlans._.media
      den.aspects.infra._.clients._.container-loki
      den.aspects.infra._.clients._.container-prometheus
      den.aspects.infra._.clients._.container-tempo
    ];

    nixos =
      { config, ... }:
      let
        cfg = config.containerConfigs.audiobookshelf;
      in
      {
        containerConfigs.audiobookshelf = {
          lldpd.enable = true;
          hostBridge = "br-media";
          macAddress = config.infra.clients.container-audiobookshelf.mac;
          localAddress = "${config.infra.clients.container-audiobookshelf.ip}/${toString config.infra.vlans.media.subnetPrefix}";
          gateway = config.infra.vlans.media.staticAddresses.gateway;
          port = config.infra.clients.container-audiobookshelf.ports.default;

          configuratarr = {
            enable = true;
            waitForHealthy = true;
            settings.audiobookshelf = {
              type = "audiobookshelf-v1";
              url = "http://localhost:${toString config.infra.clients.container-audiobookshelf.ports.default}";
              # Named `token`, not `api_key`: audiobookshelf authenticates with a
              # bearer token (the crate is explicit that it is "named `token`
              # rather than `api_key` because that is what it is").
              token = "\${env.AUDIOBOOKSHELF_API_KEY}";
              libraries = [
                {
                  name = "Audiobooks";
                  # Where Bindery lands its hardlinks (audiobookDir below).
                  # Audiobookshelf's scanner is read-only on the audio files;
                  # the one operation that writes tags in place is the manual
                  # Tools → Embed Metadata action, which has no API surface to
                  # pin here — see the warning in containers/audiobookshelf.nix.
                  folders = [ { full_path = "/mnt/media/library/audiobooks"; } ];
                  media_type = "book";
                }
              ];
              # Single sign-on. auth_open_id_auto_launch skips Audiobookshelf's
              # own login form and redirects straight to Authelia; with an
              # existing Authelia session that is a silent round-trip, so there
              # is no second password to type. auto_register creates the local
              # account on first login and match_existing_by ties it to the
              # `admin` account the setup script already made, rather than
              # spawning a duplicate.
              server_settings = {
                auth_active_auth_methods = [
                  "local"
                  "openid"
                ];
                auth_open_id_issuer_url = "https://authelia.caddy.${config.infra.vlans.apps.domain}";
                auth_open_id_authorization_url = "https://authelia.caddy.${config.infra.vlans.apps.domain}/api/oidc/authorization";
                auth_open_id_token_url = "https://authelia.caddy.${config.infra.vlans.apps.domain}/api/oidc/token";
                auth_open_id_user_info_url = "https://authelia.caddy.${config.infra.vlans.apps.domain}/api/oidc/userinfo";
                auth_open_id_jwks_url = "https://authelia.caddy.${config.infra.vlans.apps.domain}/jwks.json";
                # Without this, OidcAuthStrategy interpolates it as the literal
                # string "undefined" into the redirect_uri
                # (".../undefined/auth/openid/callback"), which Authelia rejects
                # as unregistered. It is a separate setting from ROUTER_BASE_PATH
                # and must match the subpath the frontend is built for.
                auth_open_id_subfolder_for_redirect_urls = "/audiobookshelf";
                auth_open_id_client_id = "audiobookshelf";
                auth_open_id_client_secret = "\${env.AUDIOBOOKSHELF_OIDC_CLIENT_SECRET}";
                auth_open_id_button_text = "Sign in with Authelia";
                auth_open_id_auto_launch = true;
                auth_open_id_auto_register = true;
                auth_open_id_match_existing_by = "username";
              };
            };
          };

          bindMounts = {
            "/var/lib/audiobookshelf" = {
              hostPath = "/var/lib/appdata/audiobookshelf";
              isReadOnly = false;
            };
            # Library path is added in the UI — point it at
            # /mnt/media/library/audiobooks, where Bindery lands its hardlinks.
            "/mnt/media" = {
              hostPath = "/mnt/media";
              isReadOnly = false;
            };
            # Internal CA, for the OIDC back-channel to Authelia. Same mount the
            # home-assistant container uses.
            "/run/ca/ca.crt" = {
              hostPath = toString (inputs.self + "/certs/ca.crt");
              isReadOnly = true;
            };
          };
          envFile = config.sops.templates."audiobookshelf-env".path;
          nameservers = [
            config.infra.clients.container-dns.ip
            "1.1.1.1"
          ];
          logging = {
            enable = true;
            lokiAddress = "http://${config.infra.clients.container-loki.ip}:${toString config.infra.clients.container-loki.ports.default}";
            prometheusAddress = "http://${config.infra.clients.container-prometheus.ip}:${toString config.infra.clients.container-prometheus.ports.default}/api/v1/write";
            tempoAddress = "http://${config.infra.clients.container-tempo.ip}:${toString config.infra.clients.container-tempo.ports.otlp-http}/v1/traces";
            # Audiobookshelf exposes no /metrics endpoint, so everything below is
            # derived from the journal. Counters reset when Vector restarts, so
            # query them with increase().
            prometheusInputs = [
              "audiobookshelf_scan_metrics"
              "audiobookshelf_warn_metrics"
            ];
            extraSettings.transforms = {
              audiobookshelf_scan_filter = {
                type = "filter";
                inputs = [ "journal" ];
                condition = ''match(to_string(.message) ?? "", r'(?i)\[scanner\].*(complete|finished)')'';
              };
              audiobookshelf_scan_metrics = {
                type = "log_to_metric";
                inputs = [ "audiobookshelf_scan_filter" ];
                metrics = [
                  {
                    type = "counter";
                    field = "message";
                    name = "audiobookshelf_scans_total";
                  }
                ];
              };
              audiobookshelf_warn_filter = {
                type = "filter";
                inputs = [ "journal" ];
                condition = ''match(to_string(.message) ?? "", r'\[(WARN|ERROR)\]')'';
              };
              audiobookshelf_warn_metrics = {
                type = "log_to_metric";
                inputs = [ "audiobookshelf_warn_filter" ];
                metrics = [
                  {
                    type = "counter";
                    field = "message";
                    name = "audiobookshelf_warnings_total";
                  }
                ];
              };
            };
          };
        };

        systemd.tmpfiles.rules = [
          "d /var/lib/appdata/audiobookshelf 0770 ${toString cfg.uid} ${toString cfg.gid} - -"
          "Z /var/lib/appdata/audiobookshelf 0770 ${toString cfg.uid} ${toString cfg.gid} - -"
        ];

        # Audiobookshelf tolerates an absent issuer at boot better than Komga
        # does, but its OIDC config is equally useless until Authelia answers.
        systemd.services."container@audiobookshelf" = {
          after = [ "container-wave-core.target" ];
          wants = [ "container@authelia.service" ];
        };
      };
  };
}
