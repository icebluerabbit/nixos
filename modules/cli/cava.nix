/*
  cava — terminal audio visualizer using ALSA/PipeWire; driven by FFT
  https://github.com/karlstav/cava
*/
_: {
  den.aspects.cli._.cava = {
    aspect-name = "cli.cava";
    homeManager = _: {
      stylix.targets.cava.rainbow.enable = true;
      programs.cava.enable = true;
    };
  };
}
