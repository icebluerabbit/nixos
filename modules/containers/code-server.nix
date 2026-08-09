# code-server — browser-based VS Code (coder's proxy-hardened build)
# https://github.com/coder/code-server
#
# Chosen over openvscode-server (gitpod's build) because code-server patches VS Code
# to serve webviews from the SAME origin (patches/webview.diff). openvscode-server
# keeps upstream's external vscode-cdn.net webview CDN, which is cross-origin and
# breaks the integrated terminal / webviews behind an HTTPS reverse proxy.
#
# Generic, reusable machinery: zsh + direnv terminal, host-nix-daemon devshells,
# extension symlinking, settings seeding. Host-specific choices (which language
# servers, which extensions, the settings.json) come in through the options below
# and are set by the host override.
_: {
  den.aspects.containers._.code-server = {
    aspect-name = "containers.code-server";
    includes = [ ];

    provides.to-hosts.nixos =
      { config, lib, ... }:
      let
        hostConfig = config;
      in
      {
        options.containerConfigs.code-server =
          (import ./_options.nix {
            inherit lib;
            defaultPort = 3000;
            defaultUid = 5033;
            defaultGid = 5033;
          })
          // {
            settingsFile = lib.mkOption {
              type = lib.types.nullOr lib.types.path;
              default = null;
              description = "VS Code User settings.json, written on every start (nix-owned; UI edits do not persist).";
            };
            extensions = lib.mkOption {
              type = lib.types.listOf lib.types.package;
              default = [ ];
              description = "nixpkgs vscode-extensions to symlink into the extensions dir.";
            };
            editorPackages = lib.mkOption {
              type = lib.types.listOf lib.types.package;
              default = [ ];
              description = "Extra packages on the editor PATH (language servers, tools).";
            };
          };

        config = {
          systemd.services."container@code-server" = {
            serviceConfig.MemoryHigh = lib.mkIf (
              config.containerConfigs.code-server.memoryHigh != null
            ) config.containerConfigs.code-server.memoryHigh;
          };

          containers.code-server =
            let
              cfg = hostConfig.containerConfigs.code-server;
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
                { pkgs, lib, ... }:
                let
                  dataRoot = "/var/lib/code-server";
                in
                {
                  imports = [ (import ./_base.nix { inherit lib cfg; }) ];

                  # zsh as the interactive shell; direnv hook loads .envrc into the terminal.
                  programs.zsh = {
                    enable = true;
                    interactiveShellInit = ''eval "$(${pkgs.direnv}/bin/direnv hook zsh)"'';
                  };

                  # nix-direnv's use_flake/use_nix implementation, loaded system-wide.
                  environment.etc."direnv/direnvrc".text = ''
                    source ${pkgs.nix-direnv}/share/nix-direnv/direnvrc
                  '';

                  # code-server's module creates this user as isNormalUser; we only pin
                  # the static uid/gid + home + shell (no createHome — home is bind-mounted).
                  users.users.code-server = {
                    uid = lib.mkForce cfg.uid;
                    home = lib.mkForce dataRoot;
                    createHome = lib.mkForce false;
                    shell = pkgs.zsh;
                  };
                  users.groups.code-server.gid = lib.mkForce cfg.gid;

                  services.code-server = {
                    enable = true;
                    host = "0.0.0.0";
                    inherit (cfg) port;
                    user = "code-server";
                    group = "code-server";
                    auth = "none"; # Authelia gates it at Caddy
                    disableTelemetry = true;
                    disableUpdateCheck = true;
                    userDataDir = "${dataRoot}/user-data";
                    extensionsDir = "${dataRoot}/extensions";
                    extraEnvironment = {
                      SHELL = "${pkgs.zsh}/bin/zsh";
                      # Devshell builds run through the host nix daemon (socket bind-mounted
                      # by the host override); the store is already shared into the container.
                      NIX_REMOTE = "daemon";
                      NIX_CONFIG = "experimental-features = nix-command flakes";
                    };
                    extraPackages =
                      (with pkgs; [
                        # terminal core (generic)
                        git
                        openssh # git-over-ssh transport
                        zsh
                        bashInteractive
                        ripgrep
                        fd
                        jq
                        gnumake
                        gcc
                        coreutils
                        # devshells
                        nix
                        direnv
                        nix-direnv
                      ])
                      ++ cfg.editorPackages; # host picks language servers
                  };

                  systemd.services.code-server.preStart = ''
                    extDir="${dataRoot}/extensions"
                    mkdir -p "$extDir" "${dataRoot}/user-data/User"
                    # refresh nix-managed extension symlinks; UI-installed dirs untouched
                    for l in "$extDir"/*; do
                      [ -L "$l" ] && rm -f "$l"
                    done
                    ${lib.concatMapStringsSep "\n" (ext: ''
                      for d in ${ext}/share/vscode/extensions/*; do
                        ln -sfn "$d" "$extDir/$(basename "$d")"
                      done
                    '') cfg.extensions}
                    ${lib.optionalString (cfg.settingsFile != null) ''
                      install -m0644 ${cfg.settingsFile} "${dataRoot}/user-data/User/settings.json"
                    ''}
                  '';
                };
            };
        };
      };
  };
}
