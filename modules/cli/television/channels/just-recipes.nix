{ den, ... }:
{
  den.aspects.cli._.television._.just-recipes = {
    aspect-name = "cli.television.just-recipes";
    includes = [ den.aspects.cli._.just ];

    homeManager =
      { pkgs, lib, ... }:
      let
        just = lib.getExe pkgs.just;
        tr = lib.getExe' pkgs.coreutils "tr";
      in
      {
        programs.television.channels.just-recipes = {
          metadata = {
            name = "just-recipes";
            description = "A channel to select recipes from Justfiles";
          };
          source = {
            command = "${just} --summary | ${tr} '[:blank:]' '\\n'";
          };
          preview = {
            command = "${just} -s {}";
          };
          keybindings = {
            f5 = "actions:execute-recipe";
          };
          actions = {
            execute-recipe = {
              description = "Execute a justfile recipe";
              command = "${just} {}";
              mode = "execute";
            };
          };
        };
      };
  };
}
