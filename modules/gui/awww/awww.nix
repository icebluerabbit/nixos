/*
  swww — animated Wayland wallpaper daemon using wlr-layer-shell; GPU-accelerated transitions and runtime wallpaper switching
  https://github.com/LGFae/swww
*/
_: {
  den.aspects.gui._.awww = {
    aspect-name = "gui.awww";
    homeManager = _: {
      services.awww.enable = true;
    };
  };
}
