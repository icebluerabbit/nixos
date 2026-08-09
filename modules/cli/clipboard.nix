/*
  wl-clipboard — Wayland clipboard utilities (wl-copy/wl-paste) with wl-clip-persist daemon
  https://github.com/bugaevc/wl-clipboard
*/
_: {
  den.aspects.cli._.clipboard = {
    aspect-name = "cli.clipboard";
    homeManager =
      { pkgs, lib, ... }:
      # Wayland clipboard tooling is Linux-only; macOS uses the built-in
      # pbcopy/pbpaste, so this aspect is a no-op there.
      lib.mkIf pkgs.stdenv.hostPlatform.isLinux {
        home.packages = [ pkgs.wl-clipboard ];
        services.wl-clip-persist.enable = true;
      };
  };
}
