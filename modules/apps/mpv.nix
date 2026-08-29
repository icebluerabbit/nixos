/*
  mpv — scriptable video/audio player, default handler for local media files.
  vo=gpu-next on Vulkan is the native-Wayland output path; hwdec keeps decode on
  the 4090's NVDEC block instead of the CPU.
  https://mpv.io
*/
_: {
  den.aspects.apps._.mpv = {
    aspect-name = "apps.mpv";
    homeManager =
      { pkgs, lib, ... }:
      lib.mkIf pkgs.stdenv.hostPlatform.isLinux {
        programs.mpv = {
          enable = true;
          # Default wrapper pulls yt-dlp for URL playback, and yt-dlp now drags
          # in deno as a JS runtime — 283 MiB for streaming this player is not
          # here to do. Flip to true if mpv ever needs to open a URL.
          package = pkgs.mpv.override { youtubeSupport = false; };
          config = {
            vo = "gpu-next";
            gpu-api = "vulkan";
            # auto-safe rather than a pinned nvdec: falls back to software
            # decode for codecs the card can't handle instead of failing.
            hwdec = "auto-safe";
            profile = "high-quality";
            keep-open = "yes";
            save-position-on-quit = true;
            volume = 70;
            volume-max = 130;
          };
        };

        xdg.mimeApps = {
          enable = true;
          defaultApplications = lib.genAttrs [
            "video/mp4"
            "video/mpeg"
            "video/quicktime"
            "video/webm"
            "video/x-matroska"
            "video/x-msvideo"
            "audio/flac"
            "audio/mpeg"
            "audio/ogg"
            "audio/x-wav"
          ] (_: [ "mpv.desktop" ]);
        };
      };
  };
}
