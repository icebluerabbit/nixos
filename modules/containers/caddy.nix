/*
  caddy — extensible server platform that uses TLS by default
  https://github.com/caddyserver/caddy
*/
_: {
  den.aspects.containers._.caddy = {
    aspect-name = "containers.caddy";
    includes = [ ];

    provides.to-hosts.nixos =
      { config, lib, ... }:
      {
        options.containerConfigs.caddy =
          (import ./_options.nix {
            inherit lib;
            defaultUid = 5018;
            defaultGid = 5018;
          })
          // {
            httpPort = lib.mkOption {
              type = lib.types.port;
              default = 8080;
              description = "Caddy Web UI HTTP port";
            };
            httpsPort = lib.mkOption {
              type = lib.types.port;
              default = 8443;
              description = "Caddy Web UI HTTPS port";
            };
            adminPort = lib.mkOption {
              type = lib.types.nullOr lib.types.port;
              default = 2019;
              description = "Caddy Admin API port to listen on, or null to disable";
            };
            enableTelemetry = lib.mkOption {
              type = lib.types.bool;
              default = false;
              description = "Enable OpenTelemetry tracing for Caddy HTTP requests";
            };
            virtualHosts = lib.mkOption {
              type = lib.types.attrsOf (
                lib.types.submodule {
                  options = {
                    ip = lib.mkOption { type = lib.types.str; };
                    port = lib.mkOption { type = lib.types.port; };
                    stripPrefix = lib.mkOption {
                      type = lib.types.bool;
                      default = true;
                      description = "Whether to strip the subdomain path prefix when forwarding to the backend";
                    };
                    requireAuth = lib.mkOption {
                      type = lib.types.bool;
                      default = true;
                      description = "Require Authelia forward authentication for this virtual host";
                    };
                    copyHeaders = lib.mkOption {
                      type = lib.types.listOf lib.types.str;
                      default = [
                        "Remote-User"
                        "Remote-Groups"
                        "Remote-Name"
                        "Remote-Email"
                      ];
                      description = "Headers to copy from Authelia forward_auth response to the backend request";
                    };
                    headerMap = lib.mkOption {
                      type = lib.types.attrsOf lib.types.str;
                      default = { };
                      description = "header_up renames on reverse_proxy: dest-name = source-name";
                    };
                    stream = lib.mkOption {
                      type = lib.types.bool;
                      default = false;
                      description = "Disable response buffering (flush_interval -1) for SSE/streaming backends";
                    };
                    extraDirectives = lib.mkOption {
                      type = lib.types.listOf lib.types.str;
                      default = [ ];
                      example = [
                        "@fonts path *.ttf"
                        "header @fonts Content-Type font/ttf"
                      ];
                      description = ''
                        Raw Caddyfile directives injected into this virtual host, immediately
                        before the reverse_proxy (inside the auth route when requireAuth). Use for
                        named matchers + response-header overrides, e.g. correcting a Content-Type
                        the upstream mis-serves.
                      '';
                    };
                  };
                }
              );
              default = { };
              description = "Map of subdomains (e.g., 'radarr' maps to http://radarr.internal) to backend target configurations.";
            };
            domainSuffix = lib.mkOption {
              type = lib.types.str;
              default = "server.internal";
              description = "Domain suffix for the virtual hosts (e.g. server.internal)";
            };
            tlsCertFile = lib.mkOption {
              type = lib.types.str;
              description = "Path to TLS certificate file (inside container)";
            };
            tlsKeyFile = lib.mkOption {
              type = lib.types.str;
              description = "Path to TLS private key file (inside container)";
            };
            authelia = lib.mkOption {
              type = lib.types.submodule {
                options = {
                  enable = lib.mkOption {
                    type = lib.types.bool;
                    default = false;
                  };
                  address = lib.mkOption {
                    type = lib.types.str;
                    default = "";
                    description = "Authelia address (e.g. http://10.10.140.13:9091)";
                  };
                };
              };
              default = { };
            };
          };

        config = {
          systemd.services."container@caddy" = {
            serviceConfig.MemoryHigh = lib.mkIf (
              config.containerConfigs.caddy.memoryHigh != null
            ) config.containerConfigs.caddy.memoryHigh;
          };

          containers.caddy =
            let
              cfg = config.containerConfigs.caddy;
            in
            {
              inherit (cfg) autoStart;
              ephemeral = true;
              timeoutStartSec = "5min";
              privateNetwork = true;
              inherit (cfg) hostBridge;
              localMacAddress = cfg.macAddress;
              inherit (cfg) localAddress;
              inherit (cfg) bindMounts;

              config =
                { ... }:
                {
                  imports = [
                    (import ./_base.nix {
                      inherit lib cfg;
                      hasPort = false;
                    })
                  ];

                  # Internal container firewall
                  networking.firewall.allowedTCPPorts = [
                    cfg.httpPort
                    cfg.httpsPort
                  ]
                  ++ lib.optional (cfg.adminPort != null) cfg.adminPort;

                  services.caddy = {
                    enable = true;
                    globalConfig = ''
                      ${lib.optionalString (cfg.adminPort != null) "admin 0.0.0.0:${toString cfg.adminPort}"}
                      http_port ${toString cfg.httpPort}
                      https_port ${toString cfg.httpsPort}
                    '';
                    virtualHosts = lib.mapAttrs' (
                      subdomain: target:
                      let
                        headerUpLines = lib.concatMapStrings (
                          dest: "        header_up ${dest} {http.request.header.${target.headerMap.${dest}}}\n"
                        ) (lib.attrNames target.headerMap);
                        proxyDirectives = headerUpLines + lib.optionalString target.stream "        flush_interval -1\n";
                        extraDirectivesBlock = lib.concatMapStrings (d: "        ${d}\n") target.extraDirectives;
                        reverseProxyBlock =
                          if proxyDirectives == "" then
                            "      reverse_proxy http://${target.ip}:${toString target.port}"
                          else
                            ''
                                    reverse_proxy http://${target.ip}:${toString target.port} {
                              ${proxyDirectives}      }'';
                      in
                      lib.nameValuePair "https://${subdomain}.${cfg.domainSuffix}" {
                        extraConfig =
                          if (cfg.authelia.enable && target.requireAuth) then
                            ''
                              tls ${cfg.tlsCertFile} ${cfg.tlsKeyFile}
                              ${lib.optionalString cfg.enableTelemetry "tracing"}
                              route {
                                forward_auth ${cfg.authelia.address} {
                                  uri /api/authz/forward-auth
                                  copy_headers ${lib.concatStringsSep " " target.copyHeaders}
                                  # Strip the app's own Authorization header on the auth
                                  # subrequest so Authelia validates the session cookie
                                  # instead of trying (and failing) to parse it as Basic —
                                  # otherwise SPA XHRs that carry a bearer/token (Jellyfin's
                                  # MediaBrowser token) get a 401. The backend proxy below
                                  # still forwards the original Authorization untouched.
                                  header_up -Authorization
                                }
                              ${extraDirectivesBlock}${reverseProxyBlock}
                              }
                            ''
                          else
                            ''
                              tls ${cfg.tlsCertFile} ${cfg.tlsKeyFile}
                              ${lib.optionalString cfg.enableTelemetry "tracing"}
                              ${extraDirectivesBlock}reverse_proxy http://${target.ip}:${toString target.port}
                            '';
                      }
                    ) cfg.virtualHosts;
                  };

                  systemd.services.caddy.environment = lib.mkIf cfg.enableTelemetry {
                    OTEL_EXPORTER_OTLP_ENDPOINT = lib.removeSuffix "/v1/traces" cfg.logging.tempoAddress;
                    OTEL_EXPORTER_OTLP_PROTOCOL = "http/protobuf";
                    OTEL_SERVICE_NAME = "caddy";
                    OTEL_RESOURCE_ATTRIBUTES = "host.name=${config.networking.hostName},host_type=${cfg.logging.hostType}";
                  };
                };
            };
        };
      };
  };
}
