{ den, ... }:
{
  den.aspects.cli._.television._.git-branch = {
    aspect-name = "cli.television.git-branch";
    includes = [
      den.aspects.cli._.git
    ];

    homeManager =
      { pkgs, lib, ... }:
      let
        git = lib.getExe pkgs.git;
      in
      {
        programs.television.channels.git-branch = {
          metadata = {
            name = "git-branch";
          };
          preview = {
            command = "${git} show -p --stat --pretty=fuller --color=always {}";
          };
          source = {
            command = "${git} --no-pager branch --all --format=\"%(refname:short)\"";
          };
        };
      };
  };
}
