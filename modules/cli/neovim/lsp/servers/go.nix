{ den, ... }:
{
  den.aspects.cli._.neovim._.lsp._.servers._.go = {
    aspect-name = "cli.neovim.lsp.servers.go";
    includes = [ den.aspects.dev._.gopls ];

    homeManager =
      { pkgs, lib, ... }:
      {
        programs.nixvim.lsp.servers.gopls = {
          enable = true;
          package = pkgs.gopls;
          config = {
            cmd = [ (lib.getExe pkgs.gopls) ];
            filetypes = [
              "go"
              "gomod"
              "gowork"
              "gotmpl"
            ];
            # go.work first: packages/go is a workspace of several modules,
            # and rooting on the nearest go.mod instead would give gopls one
            # module at a time with no cross-module navigation.
            root_markers = [
              "go.work"
              "go.mod"
              ".git"
            ];
          };
        };
      };
  };
}
