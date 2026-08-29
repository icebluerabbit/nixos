/*
  pcmanfm-qt — LXQt's file manager, installed standalone. The runtime closure
  carries no lxqt-session/panel/openbox; only libfm-qt, Qt6 (already present via
  quickshell) and lxqt-menu-data (menu XML).

  Thumbnails: libfm-qt renders images itself and shells out to XDG
  `.thumbnailer` entries for everything else — hence ffmpegthumbnailer, which
  ships the video and audio entries. No tumbler/gvfs daemon involved.
  https://github.com/lxqt/pcmanfm-qt
*/
_: {
  den.aspects.apps._.pcmanfm-qt = {
    aspect-name = "apps.pcmanfm-qt";
    homeManager =
      { pkgs, lib, ... }:
      lib.mkIf pkgs.stdenv.hostPlatform.isLinux {
        home.packages = [
          pkgs.lxqt.pcmanfm-qt
          pkgs.ffmpegthumbnailer
          # Qt reads no icon theme from stylix, so ship the one mako already
          # uses rather than falling back to oxygen (absent → blank icons).
          pkgs.papirus-icon-theme
        ];

        # Read-only symlink into the store: preferences changed in the GUI
        # dialog are discarded on restart, every change goes through this file.
        xdg.configFile."pcmanfm-qt/default/settings.conf".text = lib.generators.toINI { } {
          Thumbnail = {
            ShowThumbnails = true;
            # KB, libfm-qt's own image thumbnailer. Upstream's 4096 skips
            # anything phone-camera sized.
            MaxThumbnailFileSize = 32768;
            # -1 = uncapped for external thumbnailers, i.e. video of any
            # size gets a frame extracted.
            MaxExternalThumbnailFileSize = -1;
            # Only skips gvfs remotes; NFS mounts count as local files.
            ThumbnailLocalFilesOnly = true;
          };
          FolderView = {
            Mode = "thumbnail";
            ThumbnailIconSize = 128;
            SortFolderFirst = true;
          };
          System = {
            Terminal = "kitty";
            FallbackIconThemeName = "Papirus-Dark";
          };
        };

        xdg.mimeApps = {
          enable = true;
          defaultApplications."inode/directory" = [ "pcmanfm-qt.desktop" ];
        };
      };
  };
}
