/*
  prettier — opinionated code formatter for web development files
  https://github.com/prettier/prettier
*/
_: {
  den.aspects.dev._.prettier = {
    aspect-name = "dev.prettier";
    homeManager =
      { pkgs, ... }:
      {
        home.packages = [ pkgs.prettier ];
      };
  };
}
