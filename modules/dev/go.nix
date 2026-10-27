/*
  Go — compiler and toolchain for the Go programming language
  https://github.com/golang/go
*/
_: {
  den.aspects.dev._.go = {
    aspect-name = "dev.go";
    homeManager =
      { pkgs, ... }:
      {
        home.packages = [ pkgs.go ];
      };
  };
}
