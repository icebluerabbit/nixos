/*
  taplo — TOML toolkit and language server
  https://github.com/tamasfe/taplo
*/
_: {
  den.aspects.dev._.taplo = {
    aspect-name = "dev.taplo";
    homeManager =
      { pkgs, ... }:
      {
        home.packages = [ pkgs.taplo ];
      };
  };
}
