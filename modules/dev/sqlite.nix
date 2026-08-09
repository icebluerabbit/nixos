/*
  sqlite — SQLite command-line tool and database engine
  https://github.com/sqlite/sqlite
*/
_: {
  den.aspects.dev._.sqlite = {
    aspect-name = "dev.sqlite";
    homeManager =
      { pkgs, ... }:
      {
        home.packages = [ pkgs.sqlite ];
      };
  };
}
