# inventory — host and user module assignment inventory
{ den, inputs, ... }:
{
  den = {
    hosts.x86_64-linux = {
      desktop = {
        aspect = den.aspects.hosts._.desktop;
        users.zms = {
          aspect = {
            includes = [
              den.aspects.users._.zms
              den.aspects.fonts._.fonts
              den.aspects.gui._.awww
              den.aspects.gui._.niri
              den.aspects.gui._.quickshell
              den.aspects.gui._.mako
              den.aspects.services._.volume-notifications
              den.aspects.apps._.chromium
              den.aspects.apps._.discord
              den.aspects.apps._.easyeffects
              den.aspects.apps._.firefox
              den.aspects.apps._.neovide
              den.aspects.apps._.kitty
              den.aspects.apps._.lm-studio
              den.aspects.apps._.mpv
              den.aspects.apps._.obs-studio
              den.aspects.apps._.pcmanfm-qt
              den.aspects.apps._.qimgv
              den.aspects.apps._.vesktop
              den.aspects.apps._.walker
              den.aspects.apps._.zed
              den.aspects.apps._.picard
              den.aspects.apps._.steam
              den.aspects.cli._.cava
              den.aspects.cli._.spotify-player
              den.aspects.cli._.whipper
            ];

            homeManager = _: {
              programs.ssh = {
                settings.server = {
                  HostName = "server.mgmt.internal";
                  User = "zms";
                  IdentityFile = "~/.ssh/id_ed25519";
                };
              };
            };

            provides.to-hosts.nixos = _: {
              services.displayManager.autoLogin = {
                enable = true;
                user = "zms";
              };
            };
          };
        };
      };
      server = {
        aspect = den.aspects.hosts._.server;
        users.zms = {
          aspect = {
            includes = [ den.aspects.users._.zms ];

            provides.to-hosts.nixos = _: {
              users.users.zms.openssh.authorizedKeys.keyFiles = [
                (inputs.self + "/pubkeys/user_desktop.pub")
                (inputs.self + "/pubkeys/user_laptop.pub")
              ];
              security.sudo.extraRules = [
                {
                  users = [ "zms" ];
                  commands = [
                    {
                      command = "/run/current-system/sw/bin/nixos-container";
                      options = [ "NOPASSWD" ];
                    }
                  ];
                }
              ];
            };
          };
        };
      };
    };
    hosts.aarch64-darwin = {
      laptop = {
        aspect = den.aspects.hosts._.laptop;
        users.zms = {
          aspect = {
            includes = [
              den.aspects.users._.zms
              den.aspects.apps._.kitty
              den.aspects.apps._.zed
              den.aspects.apps._.firefox
              # apps.discord is deliberately absent on darwin. nixpkgs'
              # darwin.nix recreates the bundle's symlinks and injects
              # modules into Contents/Resources, which invalidates the
              # signature Discord Inc. ships — AMFI then SIGKILLs it
              # (exit 137). Re-sealing needs Apple's codesign, which the
              # build sandbox does not have; darwin.sigtool signs bare files
              # only. Installed by hand from discord.com instead, like
              # Chrome and Claude.
              den.aspects.apps._.neovide
              den.aspects.apps._.spotify
            ];

            homeManager = _: {
              programs.ssh.settings.server = {
                HostName = "server.mgmt.internal";
                User = "zms";
                IdentityFile = "~/.ssh/id_ed25519";
              };
            };

            # knownHosts is a system option, not a home-manager one — the
            # desktop sets it in its own nixos section. Pinned from the repo
            # rather than learned on first connect, so a prompt-free
            # `ssh server` cannot be satisfied by whatever answers on that
            # address.
            provides.to-hosts.darwin = _: {
              programs.ssh.knownHosts.server = {
                hostNames = [ "server.mgmt.internal" ];
                publicKeyFile = inputs.self + "/pubkeys/server_host_ed25519.pub";
              };
            };
          };
        };
      };
    };
    networks.x86_64-linux.homelab = {
      aspect = den.aspects.infra._.homelab;
    };
  };
}
