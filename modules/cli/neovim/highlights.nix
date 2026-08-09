# neovim: highlights — highlight group definitions and colorscheme overrides
_: {
  den.aspects.cli._.neovim._.highlights = {
    aspect-name = "cli.neovim.highlights";
    homeManager =
      { config, ... }:
      let
        c = config.lib.stylix.colors;
      in
      {
        programs.nixvim.highlight = { };

        programs.nixvim.highlightOverride = {
          SignColumn.bg = "none";
          LineNr = {
            bg = "none";
            fg = "#${c.base03}";
          };
          LineNrAbove = {
            bg = "none";
            fg = "#${c.base03}";
          };
          LineNrBelow = {
            bg = "none";
            fg = "#${c.base03}";
          };
          CursorLineNr = {
            bg = "none";
            fg = "#${c.base05}";
          };
          FoldColumn = {
            bg = "none";
            fg = "#${c.base03}";
          };
          GitSignsAdd = {
            bg = "none";
            fg = "#${c.base0B}";
          };
          GitSignsChange = {
            bg = "none";
            fg = "#${c.base0D}";
          };
          GitSignsDelete = {
            bg = "none";
            fg = "#${c.base08}";
          };
          GitSignsUntracked = {
            bg = "none";
            fg = "#${c.base03}";
          };
          DiagnosticSignError = {
            bg = "none";
            fg = "#${c.base08}";
          };
          DiagnosticSignWarn = {
            bg = "none";
            fg = "#${c.base0A}";
          };
          DiagnosticSignHint = {
            bg = "none";
            fg = "#${c.base0C}";
          };
          DiagnosticSignInfo = {
            bg = "none";
            fg = "#${c.base0D}";
          };
        };
      };
  };
}
