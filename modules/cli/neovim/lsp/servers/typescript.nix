{ den, ... }:
{
  den.aspects.cli._.neovim._.lsp._.servers._.typescript = {
    aspect-name = "cli.neovim.lsp.servers.typescript";
    includes = [ den.aspects.dev._.typescript-language-server ];

    homeManager =
      { pkgs, lib, ... }:
      {
        programs.nixvim.lsp.servers.ts_ls = {
          enable = true;
          package = pkgs.typescript-language-server;
          config = {
            cmd = [
              (lib.getExe pkgs.typescript-language-server)
              "--stdio"
            ];
            filetypes = [
              "javascript"
              "javascriptreact"
              "javascript.jsx"
              "typescript"
              "typescriptreact"
              "typescript.jsx"
            ];
            # tsconfig.json before package.json: a monorepo package can carry
            # its own compiler options, and rooting on package.json would
            # hand tsserver the wrong ones.
            root_markers = [
              "tsconfig.json"
              "jsconfig.json"
              "package.json"
              ".git"
            ];
          };
        };
      };
  };
}
