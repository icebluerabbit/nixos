# neovim: lsp — built-in LSP client configuration (servers, capabilities, handlers, on_attach)
{ den, ... }:
{
  den.aspects.cli._.neovim._.lsp = {
    aspect-name = "cli.neovim.lsp";
    includes = [
      den.aspects.cli._.neovim._.lsp._.keymaps
      den.aspects.cli._.neovim._.lsp._.servers._.go
      den.aspects.cli._.neovim._.lsp._.servers._.json
      den.aspects.cli._.neovim._.lsp._.servers._.jsut
      den.aspects.cli._.neovim._.lsp._.servers._.lua
      den.aspects.cli._.neovim._.lsp._.servers._.markdown
      den.aspects.cli._.neovim._.lsp._.servers._.nix
      den.aspects.cli._.neovim._.lsp._.servers._.python
      den.aspects.cli._.neovim._.lsp._.servers._.typescript
      den.aspects.cli._.neovim._.lsp._.servers._.yaml
    ];

    homeManager = _: {
      programs.nixvim.lsp = {
        inlayHints.enable = true;
        luaConfig = {
          post = builtins.readFile ./config-post.lua;
          pre = builtins.readFile ./config-pre.lua;
        };
        servers."*" = {
          config = {
            root_markers = [
              ".git"
            ];
          };
        };
      };
    };
  };
}
