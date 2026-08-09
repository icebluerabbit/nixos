{ den, ... }:
{
  den.aspects.cli._.neovim._.plugins._.tiny-inline-diagnostic = {
    aspect-name = "cli.neovim.plugins.tiny-inline-diagnostic";
    includes = [ den.aspects.cli._.neovim._.lsp ];

    homeManager = _: {
      programs.nixvim.plugins.tiny-inline-diagnostic = {
        enable = true;
        lazyLoad = {
          enable = true;
          settings.event = [
            "InsertEnter"
            "BufReadPre"
            "BufReadPost"
            "BufNewFile"
            "BufEnter"
          ];
        };
        settings = {
          show_code = false;
          multilines = {
            enabled = true;
          };
          options = {
            use_icons_from_diagnostic = true;
          };
          preset = "powerline";
          virt_texts = {
            priority = 2048;
          };
        };
      };
    };
  };
}
