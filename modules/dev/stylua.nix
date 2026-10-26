/*
  StyLua — opinionated formatter for Lua
  https://github.com/JohnnyMorganz/StyLua
*/
_: {
  den.aspects.dev._.stylua = {
    aspect-name = "dev.stylua";
    homeManager =
      { pkgs, ... }:
      {
        home.packages = [ pkgs.stylua ];
      };
  };
}
