# defaults — system-wide default configuration options for NixOS and Home Manager
{ lib, den, ... }:
let
  # Binary caches, shared by every host regardless of platform. A host that
  # does not trust these rebuilds each flake input from source — on darwin
  # that meant compiling what the cache already had.
  substituters = [
    "https://cache.nixos.org"
    "https://nix-community.cachix.org"
    "https://icebluerabbit-nixos.cachix.org"
    "https://icebluerabbit-configuratarr.cachix.org"
    "https://icebluerabbit-openchamber-flake.cachix.org"
    "https://icebluerabbit-youtarr-flake.cachix.org"
    "https://icebluerabbit-denful-networks.cachix.org"
    "https://icebluerabbit-lazylibrarian.cachix.org"
  ];

  trusted-public-keys = [
    "nix-community.cachix.org-1:mB9FSh9qf2dCimDSUo8Zy7bkq5CX+/rkCWyvRCYg3Fs="
    "icebluerabbit-nixos.cachix.org-1:1EK5Mugrxcbyk8D8FKP6q3t8SeSQcMW4+IgP88mSJ/w="
    "icebluerabbit-configuratarr.cachix.org-1:dEEK2uZ8exjLoOh01aGi9GfRqnmd/DjrUUmIZmcCiu8="
    "icebluerabbit-openchamber-flake.cachix.org-1:/9N8w45z9zJxCZHQ0yaB48hAzD4x9CasbhjOB4IGwyc="
    "icebluerabbit-youtarr-flake.cachix.org-1:/SRfDLs8Un3uuJdizztndPNu6AOVNBdGgcXM7C9MLd0="
    "icebluerabbit-denful-networks.cachix.org-1:5/BJMLmXri1Qw0DIe+fAKLwPVYJPAT3yJPUc/4NIJuE="
    "icebluerabbit-lazylibrarian.cachix.org-1:AkOQOlRiZScC7nl3UWz+lw3jYWFnOU7Eon+CBvocjME="
  ];
in
{
  den = {
    default = {
      nixos = {
        system.stateVersion = "26.05";
        nixpkgs.config.allowUnfree = true;
        hardware.enableRedistributableFirmware = true;
        documentation.dev.enable = true;
        nix = {
          settings = {
            experimental-features = [
              "nix-command"
              "flakes"
            ];
            auto-optimise-store = true;
            trusted-users = [
              "root"
              "@wheel"
            ];
            inherit substituters trusted-public-keys;
          };
          gc = {
            automatic = true;
            dates = "weekly";
            options = "--delete-older-than 30d";
          };
        };
        # Where the installer drops the repo (modules/iso/installer.nix), so this
        # is right on a freshly-installed machine. A host that keeps a working
        # checkout elsewhere overrides it — otherwise `nh` and sys-mcp both read
        # the install-time snapshot forever.
        environment.variables = {
          FLAKE = lib.mkDefault "/src/nixos";
          NH_FLAKE = lib.mkDefault "/src/nixos";
        };
        home-manager = {
          useGlobalPkgs = true;
          useUserPackages = true;
          backupFileExtension = "bak";
        };
      };
      darwin = {
        nixpkgs.config.allowUnfree = true;
        nix = {
          settings = {
            experimental-features = [
              "nix-command"
              "flakes"
            ];
            trusted-users = [
              "root"
              "@admin"
            ];
            inherit substituters trusted-public-keys;
          };
          # nix-darwin takes a launchd StartCalendarInterval here rather than
          # systemd's calendar string, so this is the darwin spelling of the
          # NixOS `dates = "weekly"` above: Sunday at 03:00.
          gc = {
            automatic = true;
            interval = {
              Weekday = 0;
              Hour = 3;
              Minute = 0;
            };
            options = "--delete-older-than 30d";
          };
        };
        home-manager = {
          useGlobalPkgs = true;
          useUserPackages = true;
          backupFileExtension = "bak";
        };
      };
      homeManager = {
        manual = {
          html.enable = true;
          json.enable = true;
          manpages.enable = true;
        };
        home.stateVersion = "26.05";
        systemd.user.startServices = true;
      };
    };
    schema.user = {
      classes = lib.mkDefault [ "homeManager" ];
      includes = [ den.provides.mutual-provider ];
    };
  };
}
