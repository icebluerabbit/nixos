{ den, ... }:
{
  den.aspects.cli._.television._.git-reflog = {
    aspect-name = "cli.television.git-reflog";
    includes = [
      den.aspects.cli._.git
    ];

    homeManager =
      { pkgs, lib, ... }:
      let
        git = lib.getExe pkgs.git;
      in
      {
        programs.television.channels.git-reflog = {
          metadata = {
            name = "git-reflog";
          };
          preview = {
            command = "${git} show -p --stat --pretty=fuller --color=always {0}";
          };
          source = {
            command = "${git} reflog";
          };
        };
      };
  };
}
