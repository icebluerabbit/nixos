/*
  layout — layout and visual settings configuration for the niri compositor
  https://github.com/YaLTeR/niri
*/
_: {
  den.aspects.gui._.niri._.layout = {
    aspect-name = "gui.niri.layout";
    includes = [ ];

    homeManager = _: {
      programs.niri.settings.layout = {
        gaps = 24;
        default-column-width = {
          proportion = 0.5;
        };
        preset-column-widths = [
          { proportion = 1.0 / 3.0; }
          { proportion = 1.0 / 2.0; }
          { proportion = 2.0 / 3.0; }
        ];
        always-center-single-column = true;
        empty-workspace-above-first = false;

        tab-indicator = {
          gap = 8;
          gaps-between-tabs = 4;
          corner-radius = 8;
          width = 10;
          position = "top";
        };

        border = {
          width = 2;
          enable = false;
        };

        shadow = {
          enable = true;
          softness = 30;
          spread = 5;
          offset = {
            x = 0;
            y = 5;
          };
          color = "#0007";
        };

        struts = {
          left = 16;
          right = 16;
          top = 0;
          bottom = 0;
        };
      };
    };
  };
}
