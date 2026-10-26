/*
  yazi — async terminal file manager in Rust; Lua plugin system, built-in previewer for images/video/archives
  https://github.com/sxyazi/yazi
*/
{ den, ... }:
{
  den.aspects.cli._.yazi = {
    aspect-name = "cli.yazi";
    includes = [ den.aspects.cli._.clipboard ];

    homeManager =
      {
        config,
        lib,
        pkgs,
        ...
      }:
      let
        # glibc carries iconv on Linux; darwin has no glibc, so use libiconv.
        iconv = if pkgs.stdenv.hostPlatform.isLinux then pkgs.glibc.bin else pkgs.libiconv;
      in
      {
        # Yazi shells out for anything it cannot decode itself: video
        # thumbnails, the image formats outside the Rust image crate
        # (avif/heic/jxl/svg), PDF pages, and archive listings. Without
        # these the preview pane silently falls back to file metadata.
        home.packages = [
          pkgs.ffmpeg
          pkgs.imagemagick
          pkgs.poppler-utils
          pkgs.p7zip
          # Drives the mediainfo plugin below.
          pkgs.mediainfo
          # Reformats JSON for the preview pane, see the previewer below.
          pkgs.jq
        ];

        programs.yazi = {
          enable = true;
          enableZshIntegration = config.programs.zsh.enable;
          shellWrapperName = "y";
          initLua = ./init.lua;
          settings = {
            mgr = {
              show_symlink = true;
              show_hidden = true;
            };
            # wl-clipboard drives wl-copy, which only exists under Wayland.
            keymap.mgr.prepend_keymap =
              lib.optional pkgs.stdenv.hostPlatform.isLinux {
                on = "<C-y>";
                run = [ "plugin wl-clipboard" ];
                desc = "Copy file (or contents) to clipboard";
              }
              ++ [
                {
                  on = "<C-d>";
                  run = [ "plugin diff" ];
                  desc = "Diff the selected with the hovered file";
                }
                {
                  on = [
                    "c"
                    "m"
                  ];
                  run = [ "plugin chmod" ];
                  desc = "Chmod on selected files";
                }
                {
                  on = "l";
                  run = [ "plugin smart-enter" ];
                  desc = "Enter the child directory, or open the file";
                }
              ];
            plugin = {
              prepend_previewers = [
                {
                  url = "*.md";
                  run = "rich-preview";
                }
                # JSON is often written without any layout at all - a whole
                # document on one line - which the syntax highlighter renders
                # faithfully as a single unreadable row. jq reformats it first
                # and colours it on the way out.
                #
                # A file that does not parse falls back to its own bytes.
                # jq's complaint is discarded to get there: piper shows
                # anything on stderr in place of the preview, so leaving it
                # would replace the file with the parse error.
                {
                  url = "*.json";
                  run = ''piper -- ${lib.getExe pkgs.jq} -C . "$1" 2>/dev/null || cat "$1"'';
                }
                # .nfo is CP437 box-drawing art. Yazi's text previewer
                # reads it as UTF-8 and renders mojibake, so transcode it
                # on the way in.
                {
                  url = "*.nfo";
                  run = ''piper -- ${lib.getExe' iconv "iconv"} -f CP437 -t UTF-8 "$1"'';
                }
                # Thumbnail plus codec/bitrate/stream metadata, in place of
                # yazi's own video and audio previewers. Images are left on
                # the built-in path.
                {
                  mime = "{audio,video}/*";
                  run = "mediainfo";
                }
                {
                  mime = "application/subrip";
                  run = "mediainfo";
                }
              ];
              # Same rules again so the work happens before you land on the
              # file rather than while you wait.
              prepend_preloaders = [
                {
                  mime = "{audio,video}/*";
                  run = "mediainfo";
                }
                {
                  mime = "application/subrip";
                  run = "mediainfo";
                }
              ];
              prepend_fetchers = [
                {
                  url = "*";
                  run = "git";
                  group = "git";
                }
                {
                  url = "*/";
                  run = "git";
                  group = "git";
                }
              ];
            };
          };
          plugins = {
            inherit (pkgs.yaziPlugins)
              git
              diff
              chmod
              compress
              rich-preview
              piper
              mediainfo
              full-border
              smart-enter
              ;
          }
          // lib.optionalAttrs pkgs.stdenv.hostPlatform.isLinux {
            inherit (pkgs.yaziPlugins) wl-clipboard;
          };
        };
      };
  };
}
