/*
  qimgv — Qt image viewer, the thing pcmanfm-qt hands an image to on
  double-click (it has no built-in viewer). Shares the Qt6 and mpv libraries
  already pulled in by pcmanfm-qt and mpv, so it adds little on its own.
  https://github.com/easymodo/qimgv
*/
_: {
  den.aspects.apps._.qimgv = {
    aspect-name = "apps.qimgv";
    homeManager =
      { pkgs, lib, ... }:
      lib.mkIf pkgs.stdenv.hostPlatform.isLinux {
        # qimgv links mpv for its video/gif preview; left alone that reference
        # is the stock wrapper, which drags yt-dlp and deno back in behind the
        # trimmed mpv in apps.mpv. Same override, so both share one build.
        home.packages = [
          (pkgs.qimgv.override {
            mpv = pkgs.mpv.override { youtubeSupport = false; };
          })
        ];

        xdg.mimeApps = {
          enable = true;
          defaultApplications = lib.genAttrs [
            "image/avif"
            "image/bmp"
            "image/gif"
            "image/jpeg"
            "image/png"
            "image/tiff"
            "image/webp"
            "image/x-icon"
          ] (_: [ "qimgv.desktop" ]);
        };
      };
  };
}
