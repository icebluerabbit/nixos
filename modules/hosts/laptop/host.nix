# laptop — configuration profile for the MacBook Pro (nix-darwin, aarch64-darwin)
{ den, inputs, ... }:
let
  # Where home-manager's targets.darwin.linkApps puts the bundles for every
  # GUI package in the user profile. Dock pins point here rather than at
  # /Applications so nix owns the app, and mac-app-util keeps the path a
  # stable trampoline instead of a store symlink that moves each rebuild.
  hmApps = "/Users/zms/Applications/Home Manager Apps";
in
{
  # mac-app-util rebuilds Nix .app bundles as real trampolines instead of
  # symlinks, so Spotlight indexes them and Dock pins survive a rebuild.
  flake-file.inputs.mac-app-util = {
    url = "github:hraban/mac-app-util";
    inputs.nixpkgs.follows = "nixpkgs";
  };

  den.aspects.hosts._.laptop = {
    aspect-name = "hosts.laptop";
    includes = [
      den.provides.hostname
      den.aspects.security._.sops
      den.aspects.infra._.homelab
    ];

    darwin = _: {
      imports = [ inputs.mac-app-util.darwinModules.default ];

      nixpkgs.hostPlatform = "aarch64-darwin";

      programs.zsh.enable = true;

      time.timeZone = "America/New_York";

      # `den.provides.hostname` sets networking.hostName from the inventory
      # key; the other two are macOS-only and were left at whatever the
      # machine shipped with (ComputerName was still "mbp").
      networking = {
        computerName = "laptop";
        localHostName = "laptop";
      };

      # Touch ID for sudo, written to /etc/pam.d/sudo_local so a macOS
      # update cannot clobber it the way editing /etc/pam.d/sudo could.
      security.pam.services.sudo_local.touchIdAuth = true;

      # Working checkout on this Mac; `nh` and sys-mcp both read these.
      environment.variables = {
        FLAKE = "/Users/zms/Projects/nixos";
        NH_FLAKE = "/Users/zms/Projects/nixos";
      };

      # Captured from this machine's live `defaults` and kept to the keys that
      # actually differ from stock macOS. Every option here is `null` by
      # default in nix-darwin, which means "leave the key alone" — so only
      # what is listed is managed, and anything omitted stays hand-tunable.
      #
      # The dock's own contents (persistent-apps/-others) are deliberately NOT
      # declared: pinning them means a switch reverts any reordering done by
      # dragging, which is how a dock is normally used.
      system.defaults = {
        NSGlobalDomain = {
          AppleInterfaceStyle = "Dark";
          # Natural scrolling off — content follows the fingers upward.
          "com.apple.swipescrolldirection" = false;
        };

        dock = {
          autohide = true;
          # Bottom-right hot corner. 14 is Quick Note.
          wvous-br-corner = 14;

          # Declared, so a switch restores this order — dragging an icon
          # around now only lasts until the next one. WezTerm used to sit
          # between Firefox and Discord; it is gone with Homebrew.
          #
          # kitty comes from home-manager's app directory rather than
          # /Applications: nix installs it into the profile, and
          # targets.darwin.linkApps puts the bundle here. mac-app-util keeps
          # that path a stable trampoline instead of a store symlink that
          # moves every rebuild, which is what would otherwise break this pin.
          persistent-apps = [
            { app = "/System/Applications/Messages.app"; }
            { app = "/System/Applications/System Settings.app"; }
            # Hand-installed under /Applications: Chrome and Claude have no
            # darwin build in nixpkgs, and nixpkgs' Discord is killed by AMFI
            # for a broken signature (see the note in inventory.nix).
            { app = "/Applications/Google Chrome.app"; }
            { app = "${hmApps}/Firefox.app"; }
            { app = "/Applications/Discord.app"; }
            { app = "${hmApps}/Zed.app"; }
            { app = "/Applications/Claude.app"; }
            { app = "${hmApps}/kitty.app"; }
          ];

          persistent-others = [
            {
              folder = {
                path = "/Users/zms/Downloads";
                arrangement = "date-added";
                displayas = "stack";
                showas = "fan";
              };
            }
          ];
        };

        # Tiled windows sit flush against each other and the screen edge.
        WindowManager.EnableTiledWindowMargins = false;

        trackpad = {
          # Tap-to-click off: clicks come from actually pressing the pad.
          Clicking = false;
          TrackpadThreeFingerDrag = false;
        };

        finder = {
          FXPreferredViewStyle = "icnv";
          ShowHardDrivesOnDesktop = false;
          ShowExternalHardDrivesOnDesktop = true;
          ShowRemovableMediaOnDesktop = true;
        };

        menuExtraClock = {
          ShowDate = 0;
          ShowDayOfWeek = true;
          ShowAMPM = true;
        };

        magicmouse.MouseButtonMode = "OneButton";

        ActivityMonitor = {
          # 102 = all processes, hierarchically.
          ShowCategory = 102;
          OpenMainWindow = false;
        };
      };

      # No nix.linux-builder here on purpose. It is the only way this Mac
      # could realise an x86_64-linux derivation locally, but the module
      # ships it as a KeepAlive launchd daemon — a NixOS VM holding RAM and
      # ticking on battery from boot to shutdown. There is no demand-start
      # hook, so the alternatives are an always-on VM or starting it by
      # hand, and neither belongs on a laptop. `just eval-desktop` and
      # `eval-server` still type-check those hosts from here; building them
      # happens on the hosts themselves.
      #
      # nix-darwin release baseline; bump deliberately.
      system.stateVersion = 6;
    };
  };
}
