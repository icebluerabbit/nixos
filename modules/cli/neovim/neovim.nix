/*
  neovim — top-level aspect; fetches the nixvim flake, enables nixvim, sets EDITOR/VISUAL, and includes all neovim sub-module aspects
  https://github.com/nix-community/nixvim
*/
{ den, inputs, ... }:
{
  flake-file.inputs = {
    nixvim = {
      url = "github:nix-community/nixvim";
      inputs = {
        flake-parts.follows = "flake-parts";
        nixpkgs.follows = "nixpkgs";
      };
    };
  };

  den.aspects.cli._.neovim = {
    aspect-name = "cli.neovim";
    includes = [
      den.aspects.dev._.nodejs
      den.aspects.cli._.curl
      den.aspects.cli._.git
      den.aspects.cli._.ripgrep
      den.aspects.cli._.neovim._.auto-commands
      den.aspects.cli._.neovim._.auto-groups
      den.aspects.cli._.neovim._.clipboard
      den.aspects.cli._.neovim._.colorschemes
      den.aspects.cli._.neovim._.diagnostic
      den.aspects.cli._.neovim._.editorconfig
      den.aspects.cli._.neovim._.extra-files
      den.aspects.cli._.neovim._.extra-plugins
      den.aspects.cli._.neovim._.file-types
      den.aspects.cli._.neovim._.globals
      den.aspects.cli._.neovim._.highlights
      den.aspects.cli._.neovim._.keymaps
      den.aspects.cli._.neovim._.lsp
      den.aspects.cli._.neovim._.options
      den.aspects.cli._.neovim._.performance
      den.aspects.cli._.neovim._.plugins._.blink-cmp
      den.aspects.cli._.neovim._.plugins._.conform
      den.aspects.cli._.neovim._.plugins._.gitsigns
      den.aspects.cli._.neovim._.plugins._.lazydev
      den.aspects.cli._.neovim._.plugins._.lualine
      den.aspects.cli._.neovim._.plugins._.lz-n
      den.aspects.cli._.neovim._.plugins._.snacks
      den.aspects.cli._.neovim._.plugins._.snacks._.bufdelete
      den.aspects.cli._.neovim._.plugins._.snacks._.dashboard
      den.aspects.cli._.neovim._.plugins._.snacks._.explorer
      den.aspects.cli._.neovim._.plugins._.snacks._.indent
      den.aspects.cli._.neovim._.plugins._.snacks._.input
      den.aspects.cli._.neovim._.plugins._.snacks._.lazygit
      den.aspects.cli._.neovim._.plugins._.snacks._.notifier
      den.aspects.cli._.neovim._.plugins._.snacks._.picker
      den.aspects.cli._.neovim._.plugins._.snacks._.rename
      den.aspects.cli._.neovim._.plugins._.snacks._.scope
      den.aspects.cli._.neovim._.plugins._.snacks._.scratch
      den.aspects.cli._.neovim._.plugins._.snacks._.scroll
      den.aspects.cli._.neovim._.plugins._.snacks._.statuscolumn
      den.aspects.cli._.neovim._.plugins._.snacks._.terminal
      den.aspects.cli._.neovim._.plugins._.snacks._.words
      den.aspects.cli._.neovim._.plugins._.snacks._.zen
      den.aspects.cli._.neovim._.plugins._.tiny-inline-diagnostic
      den.aspects.cli._.neovim._.plugins._.todo-comments
      den.aspects.cli._.neovim._.plugins._.treesitter
      # den.aspects.cli._.neovim._.plugins._.treesitter-context
      den.aspects.cli._.neovim._.plugins._.treesitter-textobjects
      den.aspects.cli._.neovim._.plugins._.which-key
      den.aspects.cli._.neovim._.plugins._.yanky
      den.aspects.cli._.neovim._.user-commands
    ];

    nixos = _: {
      programs.neovim = {
        enable = true;
        defaultEditor = true;
        viAlias = true;
        vimAlias = true;
      };
    };

    homeManager =
      { ... }:
      {
        imports = [ inputs.nixvim.homeModules.nixvim ];
        programs.nixvim = {
          enable = true;
          defaultEditor = true;
          nixpkgs.source = inputs.nixpkgs;
        };
        home.sessionVariables = {
          EDITOR = "nvim";
          VISUAL = "nvim";
        };
      };
  };
}
