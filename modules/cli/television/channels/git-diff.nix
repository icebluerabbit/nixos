{ den, ... }:
{
  den.aspects.cli._.television._.git-diff = {
    aspect-name = "cli.television.git-diff";
    includes = [
      den.aspects.cli._.git
    ];

    homeManager =
      { pkgs, lib, ... }:
      let
        git = lib.getExe pkgs.git;
      in
      {
        programs.television.channels.git-diff = {
          metadata = {
            name = "git-diff";
          };
          preview = {
            command = "${git} diff --color=always {}";
          };
          source = {
            command = "${git} diff --name-only";
          };
        };
      };
  };
}
