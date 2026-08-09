/*
  fd — simple, fast and user-friendly alternative to find
  https://github.com/sharkdp/fd
*/
_: {
  den.aspects.cli._.fd = {
    aspect-name = "cli.fd";
    homeManager =
      { pkgs, ... }:
      {
        home.packages = [ pkgs.fd ];
      };
  };
}
