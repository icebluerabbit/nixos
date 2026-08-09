/*
  workspaces — workspace names and layout settings configuration for the niri compositor
  https://github.com/YaLTeR/niri
*/
_: {
  den.aspects.gui._.niri._.workspaces = {
    aspect-name = "gui.niri.workspaces";
    includes = [ ];

    homeManager = _: {
      programs.niri.settings.workspaces = {
        "1".name = "one";
        "2".name = "two";
        "3".name = "three";
      };
    };
  };
}
