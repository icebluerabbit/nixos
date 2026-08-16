/*
  binds — key bindings configuration for the niri scrollable-tiling Wayland compositor
  https://github.com/YaLTeR/niri
*/
{ den, ... }:
{
  den.aspects.gui._.niri._.binds = {
    aspect-name = "gui.niri.binds";
    includes = [
      den.aspects.apps._.kitty
      den.aspects.apps._.walker
    ];

    homeManager =
      { pkgs, lib, ... }:
      {
        programs.niri.settings.binds = {
          "Mod+Return".action.spawn = [ (lib.getExe pkgs.kitty) ];
          "Mod+Space".action.spawn = [ (lib.getExe pkgs.walker) ];

          "Mod+Q".action.close-window = [ ];

          "Mod+Left".action.focus-column-left = [ ];
          "Mod+Down".action.focus-workspace-down = [ ];
          "Mod+Up".action.focus-workspace-up = [ ];
          "Mod+Right".action.focus-column-right = [ ];

          "Mod+Ctrl+Left".action.move-column-left = [ ];
          "Mod+Ctrl+Down".action.move-column-to-workspace-down = [ ];
          "Mod+Ctrl+Up".action.move-column-to-workspace-up = [ ];
          "Mod+Ctrl+Right".action.move-column-right = [ ];

          "Mod+1".action.focus-workspace = 1;
          "Mod+2".action.focus-workspace = 2;
          "Mod+3".action.focus-workspace = 3;
          "Mod+4".action.focus-workspace = 4;
          "Mod+5".action.focus-workspace = 5;
          "Mod+6".action.focus-workspace = 6;
          "Mod+7".action.focus-workspace = 7;
          "Mod+8".action.focus-workspace = 8;
          "Mod+9".action.focus-workspace = 9;

          "Mod+Ctrl+1".action.move-column-to-workspace = 1;
          "Mod+Ctrl+2".action.move-column-to-workspace = 2;
          "Mod+Ctrl+3".action.move-column-to-workspace = 3;
          "Mod+Ctrl+4".action.move-column-to-workspace = 4;
          "Mod+Ctrl+5".action.move-column-to-workspace = 5;
          "Mod+Ctrl+6".action.move-column-to-workspace = 6;
          "Mod+Ctrl+7".action.move-column-to-workspace = 7;
          "Mod+Ctrl+8".action.move-column-to-workspace = 8;
          "Mod+Ctrl+9".action.move-column-to-workspace = 9;

          "Mod+R".action.switch-preset-column-width = [ ];
          "Mod+Shift+R".action.switch-preset-window-height = [ ];
          "Mod+Ctrl+R".action.reset-window-height = [ ];

          "Mod+F".action.maximize-column = [ ];
          "Mod+Shift+F".action.fullscreen-window = [ ];
          "Mod+Ctrl+F".action.expand-column-to-available-width = [ ];

          "Mod+C".action.center-column = [ ];

          "Mod+W".action.toggle-column-tabbed-display = [ ];

          "Ctrl+Alt+Delete".action.quit = [ ];

          "XF86AudioRaiseVolume".action.spawn = [
            "wpctl"
            "set-volume"
            "@DEFAULT_AUDIO_SINK@"
            "5%+"
          ];
          "XF86AudioLowerVolume".action.spawn = [
            "wpctl"
            "set-volume"
            "@DEFAULT_AUDIO_SINK@"
            "5%-"
          ];
          "XF86AudioMute".action.spawn = [
            "wpctl"
            "set-mute"
            "@DEFAULT_AUDIO_SINK@"
            "toggle"
          ];
        };
      };
  };
}
