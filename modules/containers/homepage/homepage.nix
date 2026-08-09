/*
  homepage — modern self-hosted services dashboard
  https://gethomepage.dev

  This aspect owns the container runtime + the theme/style layer: it binds the
  host's stylix base16 palette into Homepage's CSS custom properties and appends
  the static restyle in ./custom.css and polish in ./custom.js. All dashboard
  *content* (services, widgets, bookmarks, settings) is set by the host override
  in modules/hosts/server/containers/homepage/.
*/
{ den, ... }:
{
  den.aspects.containers._.homepage = {
    aspect-name = "containers.homepage";
    includes = [ den.aspects.style._.stylix ];

    provides.to-hosts.nixos =
      { config, lib, ... }:
      let
        hostConfig = config;
      in
      {
        options.containerConfigs.homepage =
          (import ../_options.nix {
            inherit lib;
            defaultPort = 8082;
          })
          // {
            allowedHosts = lib.mkOption {
              type = lib.types.listOf lib.types.str;
              default = [
                "homepage.caddy.server.internal"
                "homepage.server.internal"
              ];
              description = "Hostnames under which Homepage runs";
            };
            widgets = lib.mkOption {
              type = lib.types.listOf lib.types.attrs;
              default = [ ];
              description = "Homepage widgets configuration";
            };
            services = lib.mkOption {
              type = lib.types.listOf lib.types.attrs;
              default = [ ];
              description = "Homepage services configuration";
            };
            bookmarks = lib.mkOption {
              type = lib.types.listOf lib.types.attrs;
              default = [ ];
              description = "Homepage bookmarks configuration";
            };
            settings = lib.mkOption {
              type = lib.types.attrs;
              default = { };
              description = "Homepage settings configuration";
            };
            envFile = lib.mkOption {
              type = lib.types.nullOr lib.types.path;
              default = null;
              description = "Host path to the environment file containing secrets to be mounted and loaded";
            };
            customCSS = lib.mkOption {
              type = lib.types.nullOr lib.types.str;
              default = null;
              description = "Extra custom CSS appended after the stylix palette binding and ./custom.css";
            };
            customJS = lib.mkOption {
              type = lib.types.nullOr lib.types.str;
              default = null;
              description = "Extra custom JavaScript appended after ./custom.js";
            };
          };

        config = {
          systemd.services."container@homepage" = {
            serviceConfig.MemoryHigh = lib.mkIf (
              config.containerConfigs.homepage.memoryHigh != null
            ) config.containerConfigs.homepage.memoryHigh;
          };

          containers.homepage =
            let
              cfg = hostConfig.containerConfigs.homepage;
              inherit (cfg) port;
            in
            {
              inherit (cfg) autoStart;
              ephemeral = true;
              timeoutStartSec = "5min";
              privateNetwork = true;
              inherit (cfg) hostBridge;
              localMacAddress = cfg.macAddress;
              inherit (cfg) localAddress;
              bindMounts =
                cfg.bindMounts
                // (lib.optionalAttrs (cfg.envFile != null) {
                  "/run/secrets/homepage-env" = {
                    hostPath = cfg.envFile;
                    isReadOnly = true;
                  };
                });

              config =
                {
                  lib,
                  ...
                }:
                let
                  # Bind the host's stylix base16 palette into the CSS custom
                  # properties Homepage (and ./custom.css) consume. Colors are
                  # emitted as space-separated "r g b" triples so they compose
                  # with rgb()/rgba() in the stylesheet.
                  stylixCss = lib.optionalString (hostConfig.stylix.enable or false) (
                    let
                      c = hostConfig.lib.stylix.colors;
                      toRgb = base: "${c."${base}-rgb-r"} ${c."${base}-rgb-g"} ${c."${base}-rgb-b"}";
                      sansFont = hostConfig.stylix.fonts.sansSerif.name;
                      monoFont = hostConfig.stylix.fonts.monospace.name;
                    in
                    ''
                      :root,
                      body,
                      .theme-white,
                      .theme-slate,
                      .theme-zinc,
                      .theme-neutral,
                      .theme-gray,
                      .theme-black {
                        --color-50: ${toRgb "base07"} !important;
                        --color-100: ${toRgb "base06"} !important;
                        --color-200: ${toRgb "base05"} !important;
                        --color-300: ${toRgb "base04"} !important;
                        --color-400: ${toRgb "base03"} !important;
                        --color-500: ${toRgb "base02"} !important;
                        --color-600: ${toRgb "base02"} !important;
                        --color-700: ${toRgb "base01"} !important;
                        --color-800: ${toRgb "base00"} !important;
                        --color-900: ${toRgb "base00"} !important;

                        --color-logo-start: ${toRgb "base0E"} !important;
                        --color-logo-stop: ${toRgb "base0D"} !important;

                        /* Full base16 accent palette for ./custom.css */
                        --base-bg: ${toRgb "base00"};
                        --base-bg-alt: ${toRgb "base01"};
                        --base-surface: ${toRgb "base02"};
                        --base-muted: ${toRgb "base03"};
                        --base-fg-dim: ${toRgb "base04"};
                        --base-fg: ${toRgb "base05"};
                        --base-fg-bright: ${toRgb "base06"};
                        --base-fg-brightest: ${toRgb "base07"};
                        --accent-red: ${toRgb "base08"};
                        --accent-orange: ${toRgb "base09"};
                        --accent-yellow: ${toRgb "base0A"};
                        --accent-green: ${toRgb "base0B"};
                        --accent-cyan: ${toRgb "base0C"};
                        --accent-blue: ${toRgb "base0D"};
                        --accent-magenta: ${toRgb "base0E"};
                        --accent-brown: ${toRgb "base0F"};

                        --font-sans: '${sansFont}';
                        --font-mono: '${monoFont}';
                      }

                      body {
                        font-family: '${sansFont}', sans-serif !important;
                      }

                      code, pre {
                        font-family: '${monoFont}', monospace !important;
                      }
                    ''
                  );

                  restyleCss = builtins.readFile ./custom.css;
                  polishJs = builtins.readFile ./custom.js;

                  userCss = if (cfg.customCSS != null) then cfg.customCSS else "";
                  userJs = if (cfg.customJS != null) then cfg.customJS else "";
                in
                {
                  imports = [
                    (import ../_base.nix { inherit lib cfg; })
                  ];

                  services.homepage-dashboard = {
                    enable = true;
                    listenPort = port;
                    openFirewall = false;
                    allowedHosts = lib.concatStringsSep "," cfg.allowedHosts;
                    inherit (cfg) widgets;
                    inherit (cfg) services;
                    inherit (cfg) bookmarks;
                    inherit (cfg) settings;
                    customCSS = "${stylixCss}\n${restyleCss}\n${userCss}";
                    customJS = "${polishJs}\n${userJs}";
                  };

                  systemd.services.homepage-dashboard.serviceConfig.EnvironmentFile = lib.mkIf (
                    cfg.envFile != null
                  ) "/run/secrets/homepage-env";

                };
            };
        };
      };
  };
}
