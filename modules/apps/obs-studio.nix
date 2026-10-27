/*
  obs-studio — screen recording and streaming.

  Capture needs no extra wiring: niri-flake already pulls in
  xdg-desktop-portal-gnome, and niri-portals.conf sends ScreenCast there (niri
  speaks the Mutter ScreenCast API), so OBS's "Screen Capture (PipeWire)"
  source works as-is — no wlrobs, no portal overrides. NVENC comes from the
  nvidia driver already on this host.
  https://obsproject.com
*/
_: {
  den.aspects.apps._.obs-studio = {
    aspect-name = "apps.obs-studio";
    homeManager =
      { pkgs, lib, ... }:
      let
        # Browser Source and browser docks are a bundled Chromium: cef-binary
        # is 1.95 GiB of the stock package's 2.31 GiB. Dropping it costs stream
        # overlays/alert boxes and makes obs-studio a local build rather than a
        # cache hit.
        obs-studio = pkgs.obs-studio.override { browserSupport = false; };
      in
      lib.mkIf pkgs.stdenv.hostPlatform.isLinux {
        programs.obs-studio = {
          package = obs-studio;
          enable = true;
          plugins = [
            # PipeWire exposes no per-application monitor source, so without
            # this OBS can only capture the whole sink. Built against the same
            # override — the stock plugin would drag the CEF-carrying build
            # back into the closure alongside this one.
            (pkgs.obs-studio-plugins.obs-pipewire-audio-capture.override {
              inherit obs-studio;
            })
          ];
        };
      };
  };
}
