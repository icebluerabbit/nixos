/*
  gopls — Language Server Protocol (LSP) server for Go
  https://github.com/golang/tools/tree/master/gopls

  Includes dev.go rather than assuming a toolchain: gopls shells out to `go`
  for module resolution, build metadata and diagnostics, and degrades to little
  more than syntax highlighting without one on PATH.
*/
{ den, ... }:
{
  den.aspects.dev._.gopls = {
    aspect-name = "dev.gopls";
    includes = [ den.aspects.dev._.go ];
    homeManager =
      { pkgs, ... }:
      {
        home.packages = [ pkgs.gopls ];
      };
  };
}
