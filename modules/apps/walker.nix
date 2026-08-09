/*
  Walker — multi-purpose Wayland application launcher written in Go with GTK4
  https://github.com/abenz1267/walker
  Runs as a systemd user service (daemon mode) backed by the Elephant provider service.
*/
{ den, ... }:
{
  den.aspects.apps._.walker = {
    aspect-name = "apps.walker";
    includes = [
      den.aspects.services._.elephant
    ];

    homeManager = _: {
      services.walker = {
        enable = true;
        systemd.enable = true;
      };
    };
  };
}
