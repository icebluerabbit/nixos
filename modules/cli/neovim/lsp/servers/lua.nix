{ den, ... }:
{
  den.aspects.cli._.neovim._.lsp._.servers._.lua = {
    aspect-name = "cli.neovim.lsp.servers.lua";
    includes = [ den.aspects.dev._.lua-language-server ];

    homeManager =
      { pkgs, lib, ... }:
      {
        programs.nixvim.lsp.servers.lua_ls = {
          enable = true;
          package = pkgs.lua-language-server;
          config = {
            cmd = [ (lib.getExe pkgs.lua-language-server) ];
            filetypes = [ "lua" ];
            root_markers = [
              ".luarc.json"
              ".luarc.jsonc"
              ".luacheckrc"
              ".stylua.toml"
              "stylua.toml"
              "selene.toml"
              "selene.yml"
              ".git"
            ];
            settings = {
              Lua = {
                diagnostics = {
                  globals = [ "vim" ];
                };
              };
            };
          };
        };
      };
  };
}
