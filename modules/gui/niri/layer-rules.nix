/*
  layer-rules — Wayland layer rules configuration for the niri compositor
  https://github.com/YaLTeR/niri
*/
_: {
  den.aspects.gui._.niri._.layer-rules = {
    aspect-name = "gui.niri.layer-rules";
    includes = [ ];

    homeManager = _: {
      programs.niri.settings.layer-rules = [
        {
          matches = [
            { namespace = "^quickshell$"; }
          ];
          place-within-backdrop = true;
        }
      ];
    };
  };
}
