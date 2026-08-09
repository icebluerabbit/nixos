# neovim: diagnostic — vim.diagnostic settings (signs, virtual text, underline, update behavior)
_: {
  den.aspects.cli._.neovim._.diagnostic = {
    aspect-name = "cli.neovim.diagnostic";
    homeManager = _: {
      programs.nixvim.diagnostic = {
        settings = {
          signs = {
            text = {
              "__rawKey__vim.diagnostic.severity.ERROR" = "󰅚";
              "__rawKey__vim.diagnostic.severity.WARN" = "󰀪";
              "__rawKey__vim.diagnostic.severity.HINT" = "󰌶";
              "__rawKey__vim.diagnostic.severity.INFO" = "󰋽";
            };
          };
          virtual_text = false;
          underline = true;
          update_in_insert = false;
          severity_sort = true;
        };
      };
    };
  };
}
