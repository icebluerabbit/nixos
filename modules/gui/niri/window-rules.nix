/*
  window-rules — window appearance and behavior rules configuration for the niri compositor
  https://github.com/YaLTeR/niri
*/
_: {
  den.aspects.gui._.niri._.window-rules = {
    aspect-name = "gui.niri.window-rules";
    includes = [ ];

    homeManager = _: {
      programs.niri.settings.window-rules = [
        {
          geometry-corner-radius = {
            top-left = 12.0;
            top-right = 12.0;
            bottom-left = 12.0;
            bottom-right = 12.0;
          };
          clip-to-geometry = true;
        }
        {
          matches = [
            { is-active = false; }
          ];
          opacity = 0.95;
          # background-effect = {
          #   blur = true;
          #   xray = true;
          # };
        }
        {
          # Steam games run through XWayland and their fullscreen requests
          # land as plain toplevels, so niri tiles them like any other window.
          # Force fullscreen and opt them out of the inactive-window dimming.
          matches = [
            { app-id = "^steam_app_[0-9]+$"; }
          ];
          open-fullscreen = true;
          opacity = 1.0;
          clip-to-geometry = false;
        }
      ];
    };
  };
}
