{ den, ... }:
{
  den.aspects.cli._.television._.git-log = {
    aspect-name = "cli.television.git-log";
    includes = [
      den.aspects.cli._.git
    ];

    homeManager =
      { pkgs, lib, ... }:
      let
        git = lib.getExe pkgs.git;
      in
      {
        programs.television.channels.git-log = {
          metadata = {
            name = "git-log";
          };
          preview = {
            command = "${git} show -p --stat --pretty=fuller --color=always {0}";
          };
          source = {
            command = "${git} log --oneline --date=short --pretty=\"format:%h %s %an %cd\"";
          };
        };
      };
  };
}
