/*
  psmisc — system utilities using the proc filesystem, such as killall, fuser, and pstree
  https://gitlab.com/psmisc/psmisc
*/
_: {
  den.aspects.cli._.psmisc = {
    aspect-name = "cli.psmisc";
    homeManager =
      { pkgs, lib, ... }:
      # psmisc (killall/fuser/pstree) is a procfs tool, Linux-only.
      lib.mkIf pkgs.stdenv.hostPlatform.isLinux {
        home.packages = [ pkgs.psmisc ];
      };
  };
}
