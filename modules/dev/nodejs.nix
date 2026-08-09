/*
  Node.js — JavaScript runtime built on V8; used for server-side and frontend tooling
  https://github.com/nodejs/node
*/
_: {
  den.aspects.dev._.nodejs = {
    aspect-name = "dev.nodejs";
    homeManager =
      { pkgs, ... }:
      {
        home.packages = [ pkgs.nodejs ];
      };
  };
}
