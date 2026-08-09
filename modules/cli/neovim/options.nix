# neovim: options — vim.opt settings (opts, localOpts, globalOpts) for editor behavior
_: {
  den.aspects.cli._.neovim._.options = {
    aspect-name = "cli.neovim.options";
    homeManager = _: {
      programs.nixvim = {
        opts = {
          number = true; # Show line numbers
          relativenumber = true; # Show relative line numbers

          clipboard = "unnamedplus"; # Use the system clipboard

          scrolloff = 5; # Keep 5 lines above and below the cursor

          ignorecase = true; # Ignore case when searching
          smartcase = true; # Ignore case when searching, unless an uppercase letter is used
          autoindent = true; # Automatically indent new lines
          smartindent = true; # Automatically indent new lines based on the previous line

          tabstop = 2; # Number of spaces a tab counts for
          shiftwidth = 2; # Number of spaces to use for autoindent
          expandtab = true; # Use spaces instead of tabs
          smarttab = true; # Use shiftwidth when tabbing

          cursorline = true; # Highlight the current line
        };
        localOpts = { };
        globalOpts = { };
      };
    };
  };
}
